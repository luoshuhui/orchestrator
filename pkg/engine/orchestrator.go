package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"orchestrator/pkg/api"
	"orchestrator/pkg/dag"
	"orchestrator/pkg/eventbus"
)

// StateChangeEvent 状态变化事件
type StateChangeEvent struct {
	ResourceID api.ResourceID
	OldState   api.ResourceState
	NewState   api.ResourceState
	Output     api.ProvisionOutputs
	Error      error
}

// Orchestrator 编排引擎核心
type Orchestrator struct {
	dag         *dag.Graph
	stateStore  api.StateStore
	eventBus    eventbus.EventBus
	gateManager *GateManager

	// 状态通道
	stateChangeCh chan StateChangeEvent

	// 控制通道
	stopCh chan struct{}

	// 资源映射
	resources map[api.ResourceID]api.Resource

	// 执行状态
	completed map[api.ResourceID]bool
	outputs   map[api.ResourceID]api.ProvisionOutputs
	mu        sync.RWMutex

	// 并发控制
	maxConcurrent int
	sem           chan struct{}

	// 工作流信息
	workflowID string
}

// NewOrchestrator 创建编排引擎实例
func NewOrchestrator(
	workflowID string,
	dag *dag.Graph,
	stateStore api.StateStore,
	eventBus eventbus.EventBus,
	maxConcurrent int,
) *Orchestrator {
	return &Orchestrator{
		workflowID:     workflowID,
		dag:            dag,
		stateStore:     stateStore,
		eventBus:       eventBus,
		gateManager:    NewGateManager(eventBus),
		stateChangeCh:  make(chan StateChangeEvent, 256),
		stopCh:         make(chan struct{}),
		resources:      make(map[api.ResourceID]api.Resource),
		completed:      make(map[api.ResourceID]bool),
		outputs:        make(map[api.ResourceID]api.ProvisionOutputs),
		maxConcurrent:  maxConcurrent,
		sem:            make(chan struct{}, maxConcurrent),
	}
}

// RegisterResource 注册资源
func (o *Orchestrator) RegisterResource(r api.Resource) {
	o.resources[r.Identity()] = r
}

// Run 启动编排引擎
func (o *Orchestrator) Run(ctx context.Context) error {
	// 1. 验证 DAG
	if err := o.dag.Validate(); err != nil {
		return fmt.Errorf("DAG validation failed: %w", err)
	}

	// 2. 预先为所有有事件依赖的节点创建门控
	// 这样可以确保在事件发布时，门控已经准备好接收
	for _, nodeID := range o.dag.AllNodes() {
		node, _ := o.dag.GetNode(nodeID)
		if len(node.RequiredEvents) > 0 {
			o.gateManager.CreateGate(nodeID, node.RequiredEvents)
		}
	}

	// 3. 恢复未完成的工作流状态
	if err := o.recover(ctx); err != nil {
		return fmt.Errorf("recovery failed: %w", err)
	}

	// 4. 启动事件监听
	go o.watchEvents(ctx)

	// 5. 启动主 Reconcile 循环
	return o.reconcileLoop(ctx)
}

// recover 恢复未完成的工作流
func (o *Orchestrator) recover(ctx context.Context) error {
	if o.stateStore == nil {
		return nil
	}

	snapshot, err := o.stateStore.LoadWorkflow(ctx, o.workflowID)
	if err != nil || snapshot == nil {
		return nil // 没有需要恢复的状态
	}

	// 恢复已完成状态
	for id, res := range snapshot.Resources {
		if res.State == api.StateSucceeded {
			o.completed[id] = true
			o.outputs[id] = res.Outputs
		}
	}

	return nil
}

// watchEvents 监听事件
func (o *Orchestrator) watchEvents(ctx context.Context) {
	sub := o.eventBus.Subscribe(eventbus.MatchAny())
	defer o.eventBus.Unsubscribe(sub)

	for {
		select {
		case event := <-sub.EventCh:
			o.handleEvent(event)
		case <-ctx.Done():
			return
		case <-o.stopCh:
			return
		}
	}
}

// handleEvent 处理事件
func (o *Orchestrator) handleEvent(event api.Event) {
	// 事件会被 GateManager 自动转发到对应的门控
	// 这里可以添加额外的事件处理逻辑
}

// reconcileLoop 主协调循环
func (o *Orchestrator) reconcileLoop(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-o.stopCh:
			return nil
		case <-ticker.C:
			if err := o.reconcile(ctx); err != nil {
				return err
			}
		case change := <-o.stateChangeCh:
			o.handleStateChange(ctx, change)
		}
	}
}

// reconcile 执行一次协调
func (o *Orchestrator) reconcile(ctx context.Context) error {
	// 获取就绪节点
	readyNodes := o.dag.GetReadyNodes(o.completed)

	// 并行执行就绪节点
	var wg sync.WaitGroup
	for _, nodeID := range readyNodes {
		node, exists := o.dag.GetNode(nodeID)
		if !exists {
			continue
		}

		// 检查事件门控
		if len(node.RequiredEvents) > 0 {
			gate := o.gateManager.GetGate(nodeID)
			if gate == nil {
				gate = o.gateManager.CreateGate(nodeID, node.RequiredEvents)
			}

			status := gate.Status()
			if status.State != GateStateOpen {
				// 标记为 Blocked 状态
				o.updateState(nodeID, api.StateBlocked)
				// 非阻塞等待事件
				go func(id api.ResourceID, g EventGate) {
					events, err := g.Wait(ctx)
					if err == nil && len(events) > 0 {
						// 事件满足，触发立即协调
						o.stateChangeCh <- StateChangeEvent{
							ResourceID: id,
							NewState:   api.StatePending,
						}
					}
				}(nodeID, gate)
				continue // 事件门控未打开，跳过
			}
		}

		wg.Add(1)
		go func(id api.ResourceID) {
			defer wg.Done()
			o.executeNode(ctx, id)
		}(nodeID)
	}
	wg.Wait()

	// 检查是否全部完成
	if o.isComplete() {
		return nil
	}

	return nil
}

// executeNode 执行单个节点
func (o *Orchestrator) executeNode(ctx context.Context, nodeID api.ResourceID) {
	// 获取信号量
	select {
	case o.sem <- struct{}{}:
		defer func() { <-o.sem }()
	case <-ctx.Done():
		return
	}

	_, exists := o.dag.GetNode(nodeID)
	if !exists {
		return
	}

	// 更新状态为 Running
	o.updateState(nodeID, api.StateRunning)

	// 构建输入
	inputs := o.buildInputs(nodeID)

	// 执行 Provision
	resource := o.resources[nodeID]
	outputs, err := resource.Provision(ctx, inputs)

	if err != nil {
		o.updateState(nodeID, api.StateFailed)
		o.stateChangeCh <- StateChangeEvent{
			ResourceID: nodeID,
			NewState:   api.StateFailed,
			Error:      err,
		}
		return
	}

	// 保存输出
	o.mu.Lock()
	o.outputs[nodeID] = outputs
	o.completed[nodeID] = true
	o.mu.Unlock()

	// 发布成功事件
	for _, event := range outputs.Events {
		o.eventBus.Publish(event)
	}

	// 更新状态
	o.updateState(nodeID, api.StateSucceeded)
	o.stateChangeCh <- StateChangeEvent{
		ResourceID: nodeID,
		NewState:   api.StateSucceeded,
		Output:     outputs,
	}

	// 清理门控
	o.gateManager.RemoveGate(nodeID)
}

// buildInputs 构建节点输入
func (o *Orchestrator) buildInputs(nodeID api.ResourceID) api.ProvisionInputs {
	node, _ := o.dag.GetNode(nodeID)

	inputs := api.ProvisionInputs{
		UpstreamOutputs: make(map[api.ResourceID]api.ProvisionOutputs),
		EventPayloads:   make(map[string]map[string]interface{}),
		Context:         make(map[string]interface{}),
	}

	// 收集上游输出
	o.mu.RLock()
	for _, dep := range node.Dependencies {
		if output, exists := o.outputs[dep]; exists {
			inputs.UpstreamOutputs[dep] = output
		}
	}
	o.mu.RUnlock()

	// 收集事件负载
	if gate := o.gateManager.GetGate(nodeID); gate != nil {
		status := gate.Status()
		for i, event := range status.ReceivedEvents {
			name := node.RequiredEvents[i].Name
			if name == "" {
				name = string(event.Type)
			}
			inputs.EventPayloads[name] = event.Payload
		}
	}

	return inputs
}

// updateState 更新资源状态
func (o *Orchestrator) updateState(nodeID api.ResourceID, state api.ResourceState) {
	// 状态更新通过资源自身的方法处理
	// 这里可以添加额外的状态更新逻辑
}

// handleStateChange 处理状态变化
func (o *Orchestrator) handleStateChange(ctx context.Context, change StateChangeEvent) {
	// 持久化状态
	if o.stateStore != nil {
		snapshot := api.ResourceSnapshot{
			ID:      change.ResourceID,
			State:   change.NewState,
			Outputs: change.Output,
		}
		if change.Error != nil {
			snapshot.Error = change.Error.Error()
		}
		o.stateStore.SaveResource(ctx, o.workflowID, snapshot)
	}
}

// isComplete 检查是否全部完成
func (o *Orchestrator) isComplete() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return len(o.completed) == o.dag.Size()
}

// Stop 停止编排引擎
func (o *Orchestrator) Stop() {
	close(o.stopCh)
	o.gateManager.Close()
}

// GetOutputs 获取所有输出
func (o *Orchestrator) GetOutputs() map[api.ResourceID]api.ProvisionOutputs {
	o.mu.RLock()
	defer o.mu.RUnlock()

	result := make(map[api.ResourceID]api.ProvisionOutputs)
	for k, v := range o.outputs {
		result[k] = v
	}
	return result
}
