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

type StateChangeEvent struct {
	ResourceID api.ResourceID
	NewState   api.ResourceState
	Output     api.ProvisionOutputs
	Error      error
}

type Orchestrator struct {
	dag         *dag.Graph
	stateStore  api.StateStore
	eventBus    eventbus.EventBus
	gateManager *GateManager

	stateChangeCh chan StateChangeEvent
	stopCh        chan struct{}
	resources     map[api.ResourceID]api.Resource
	completed     map[api.ResourceID]bool
	active        map[api.ResourceID]bool // 正在运行或等待的节点
	outputs       map[api.ResourceID]api.ProvisionOutputs
	mu            sync.RWMutex

	maxConcurrent int
	sem           chan struct{}
	workflowID    string
}

func NewOrchestrator(workflowID string, dag *dag.Graph, stateStore api.StateStore, bus eventbus.EventBus, max int) *Orchestrator {
	return &Orchestrator{
		workflowID:    workflowID,
		dag:           dag,
		stateStore:    stateStore,
		eventBus:      bus,
		gateManager:   NewGateManager(bus),
		stateChangeCh: make(chan StateChangeEvent, 256),
		stopCh:        make(chan struct{}),
		resources:     make(map[api.ResourceID]api.Resource),
		completed:     make(map[api.ResourceID]bool),
		active:        make(map[api.ResourceID]bool),
		outputs:       make(map[api.ResourceID]api.ProvisionOutputs),
		maxConcurrent: max,
		sem:           make(chan struct{}, max),
	}
}

func (o *Orchestrator) RegisterResource(r api.Resource) {
	o.resources[r.Identity()] = r
}

func (o *Orchestrator) Run(ctx context.Context) error {
	if err := o.dag.Validate(); err != nil {
		return fmt.Errorf("DAG validation failed: %w", err)
	}

	for _, nodeID := range o.dag.AllNodes() {
		node, _ := o.dag.GetNode(nodeID)
		if len(node.RequiredEvents) > 0 {
			o.gateManager.CreateGate(nodeID, node.RequiredEvents)
		}
	}

	go o.watchEvents(ctx)
	return o.reconcileLoop(ctx)
}

func (o *Orchestrator) watchEvents(ctx context.Context) {
	sub := o.eventBus.Subscribe(eventbus.MatchAny())
	defer o.eventBus.Unsubscribe(sub)
	for {
		select {
		case event := <-sub.EventCh:
			// GateManager 内部会自动分发
			_ = event
		case <-ctx.Done():
			return
		case <-o.stopCh:
			return
		}
	}
}

func (o *Orchestrator) reconcileLoop(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	o.reconcile(ctx)

	for {
		if o.isComplete() {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-o.stopCh:
			return nil
		case <-ticker.C:
			o.reconcile(ctx)
		case change := <-o.stateChangeCh:
			o.handleStateChange(ctx, change)
			if o.isComplete() {
				return nil
			}
			o.reconcile(ctx)
		}
	}
}

func (o *Orchestrator) reconcile(ctx context.Context) {
	o.mu.Lock()
	readyNodes := o.dag.GetReadyNodes(o.completed)

	for _, nodeID := range readyNodes {
		if o.active[nodeID] {
			continue
		}

		node, _ := o.dag.GetNode(nodeID)

		// EDA 检查
		if len(node.RequiredEvents) > 0 {
			gate := o.gateManager.GetGate(nodeID)
			status := gate.Status()
			if status.State != GateStateOpen {
				if node.Resource.ActualState() != api.StateBlocked {
					fmt.Printf("[Orchestrator] ⏸️  Node %s blocked by event gate\n", nodeID)
					o.updateStateLocked(nodeID, api.StateBlocked)
					o.active[nodeID] = true
					// 启动监听协程
					go func(id api.ResourceID, g EventGate) {
						events, _ := g.Wait(ctx)
						if len(events) > 0 {
							fmt.Printf("[Orchestrator] 🔔 Node %s gate opened by events\n", id)
							o.stateChangeCh <- StateChangeEvent{ResourceID: id, NewState: api.StatePending}
						}
					}(nodeID, gate)
				}
				continue
			}
		}

		// 真正启动
		o.active[nodeID] = true
		go o.executeNode(ctx, nodeID)
	}
	o.mu.Unlock()
}

func (o *Orchestrator) executeNode(ctx context.Context, nodeID api.ResourceID) {
	select {
	case o.sem <- struct{}{}:
		defer func() { <-o.sem }()
	case <-ctx.Done():
		return
	}

	o.mu.Lock()
	o.updateStateLocked(nodeID, api.StateRunning)
	o.mu.Unlock()

	inputs := o.buildInputs(nodeID)
	resource := o.resources[nodeID]
	outputs, err := resource.Provision(ctx, inputs)

	o.mu.Lock()
	defer o.mu.Unlock()

	if err != nil {
		fmt.Printf("[Orchestrator] ❌ Node %s failed: %v\n", nodeID, err)
		o.updateStateLocked(nodeID, api.StateFailed)
		o.active[nodeID] = false
		o.stateChangeCh <- StateChangeEvent{ResourceID: nodeID, NewState: api.StateFailed, Error: err}
		return
	}

	o.outputs[nodeID] = outputs
	o.completed[nodeID] = true
	o.active[nodeID] = false
	o.updateStateLocked(nodeID, api.StateSucceeded)

	for _, event := range outputs.Events {
		o.eventBus.Publish(event)
	}

	o.stateChangeCh <- StateChangeEvent{ResourceID: nodeID, NewState: api.StateSucceeded, Output: outputs}
	o.gateManager.RemoveGate(nodeID)
}

func (o *Orchestrator) buildInputs(nodeID api.ResourceID) api.ProvisionInputs {
	o.mu.RLock()
	defer o.mu.RUnlock()

	node, _ := o.dag.GetNode(nodeID)
	inputs := api.ProvisionInputs{
		UpstreamOutputs: make(map[api.ResourceID]api.ProvisionOutputs),
		EventPayloads:   make(map[string]map[string]any),
		Context:         make(map[string]any),
	}
	for _, dep := range node.Dependencies {
		if output, exists := o.outputs[dep]; exists {
			inputs.UpstreamOutputs[dep] = output
		}
	}
	if gate := o.gateManager.GetGate(nodeID); gate != nil {
		status := gate.Status()
		for i, event := range status.ReceivedEvents {
			name := node.RequiredEvents[i].Name
			if name == "" { name = string(event.Type) }
			inputs.EventPayloads[name] = event.Payload
		}
	}
	return inputs
}

func (o *Orchestrator) updateStateLocked(nodeID api.ResourceID, state api.ResourceState) {
	if res, ok := o.resources[nodeID].(interface{ SetState(api.ResourceState) }); ok {
		res.SetState(state)
	}
}

func (o *Orchestrator) handleStateChange(ctx context.Context, change StateChangeEvent) {
	o.mu.Lock()
	if change.NewState == api.StatePending {
		o.active[change.ResourceID] = false // 重置激活状态，允许重新调度
	}
	o.mu.Unlock()

	if o.stateStore != nil {
		o.stateStore.SaveResource(ctx, o.workflowID, api.ResourceSnapshot{
			ID: change.ResourceID, State: change.NewState, Outputs: change.Output,
		})
	}
}

func (o *Orchestrator) isComplete() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return len(o.completed) == o.dag.Size()
}

func (o *Orchestrator) GetOutputs() map[api.ResourceID]api.ProvisionOutputs {
	o.mu.RLock()
	defer o.mu.RUnlock()
	res := make(map[api.ResourceID]api.ProvisionOutputs)
	for k, v := range o.outputs { res[k] = v }
	return res
}
