package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"orchestrator/pkg/api"
	"orchestrator/pkg/eventbus"
)

// GateState 门控状态
type GateState string

const (
	GateStateOpen     GateState = "Open"     // 所有事件已满足
	GateStateClosed   GateState = "Closed"   // 等待事件中
	GateStateTimedOut GateState = "TimedOut" // 超时
)

// GateStatus 门控状态
type GateStatus struct {
	ResourceID     api.ResourceID
	RequiredEvents []api.RequiredEventSpec
	ReceivedEvents []api.Event
	PendingCount   int
	State          GateState
}

// EventGate 事件门控接口
// 负责管理资源的事件依赖等待与唤醒逻辑
type EventGate interface {
	// Wait 等待所有必需事件到达
	// 返回已匹配的事件列表，或超时/取消错误
	Wait(ctx context.Context) ([]api.Event, error)

	// Notify 通知事件到达
	Notify(event api.Event) bool

	// Status 返回门控状态
	Status() GateStatus

	// Close 关闭门控
	Close()
}

// eventGate 事件门控的具体实现
type eventGate struct {
	resourceID     api.ResourceID
	requiredEvents []api.RequiredEventSpec
	receivedEvents []api.Event
	eventCh        chan api.Event
	doneCh         chan struct{}
	mu             sync.Mutex
	closed         bool
}

// NewEventGate 创建新的事件门控
func NewEventGate(resourceID api.ResourceID, required []api.RequiredEventSpec) EventGate {
	return &eventGate{
		resourceID:     resourceID,
		requiredEvents: required,
		eventCh:        make(chan api.Event, 16),
		doneCh:         make(chan struct{}),
	}
}

// Wait 等待所有必需事件到达
func (g *eventGate) Wait(ctx context.Context) ([]api.Event, error) {
	// 构建待匹配事件需求
	pending := make([]api.RequiredEventSpec, 0)
	for _, spec := range g.requiredEvents {
		if spec.Required {
			pending = append(pending, spec)
		}
	}

	// 如果没有必需事件，直接返回
	if len(pending) == 0 {
		return g.receivedEvents, nil
	}

	matchedEvents := make([]api.Event, 0)

	for len(pending) > 0 {
		// 计算最早的超时时间
		deadline := g.nearestDeadline(pending)

		var timer *time.Timer
		var timerCh <-chan time.Time
		if deadline != nil && !deadline.IsZero() {
			timer = time.NewTimer(time.Until(*deadline))
			timerCh = timer.C
		}

		select {
		case event := <-g.eventCh:
			if timer != nil {
				timer.Stop()
			}
			// 尝试匹配事件
			matched := false
			for i, spec := range pending {
				if spec.Fingerprint.Match(event) {
					matchedEvents = append(matchedEvents, event)
					g.mu.Lock()
					g.receivedEvents = append(g.receivedEvents, event)
					g.mu.Unlock()
					// 移除已匹配的需求
					pending = append(pending[:i], pending[i+1:]...)
					matched = true
					break
				}
			}
			if !matched {
				// 事件不匹配，继续等待
			}

		case <-timerCh:
			// 超时
			return matchedEvents, fmt.Errorf("event gate timeout for resource %s", g.resourceID)

		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return matchedEvents, ctx.Err()

		case <-g.doneCh:
			return matchedEvents, fmt.Errorf("gate closed")
		}
	}

	return matchedEvents, nil
}

// Notify 通知事件到达
func (g *eventGate) Notify(event api.Event) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		return false
	}

	select {
	case g.eventCh <- event:
		return true
	default:
		return false
	}
}

// Status 返回门控状态
func (g *eventGate) Status() GateStatus {
	g.mu.Lock()
	defer g.mu.Unlock()

	pendingCount := 0
	for _, spec := range g.requiredEvents {
		if spec.Required {
			matched := false
			for _, event := range g.receivedEvents {
				if spec.Fingerprint.Match(event) {
					matched = true
					break
				}
			}
			if !matched {
				pendingCount++
			}
		}
	}

	state := GateStateOpen
	if pendingCount > 0 {
		state = GateStateClosed
	}

	return GateStatus{
		ResourceID:     g.resourceID,
		RequiredEvents: g.requiredEvents,
		ReceivedEvents: g.receivedEvents,
		PendingCount:   pendingCount,
		State:          state,
	}
}

// Close 关闭门控
func (g *eventGate) Close() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.closed {
		g.closed = true
		close(g.doneCh)
		close(g.eventCh)
	}
}

// nearestDeadline 找到最近的事件超时时间
func (g *eventGate) nearestDeadline(pending []api.RequiredEventSpec) *time.Time {
	var nearest *time.Time
	now := time.Now()

	for _, spec := range pending {
		if spec.Timeout > 0 {
			deadline := now.Add(spec.Timeout)
			if nearest == nil || deadline.Before(*nearest) {
				nearest = &deadline
			}
		}
	}

	return nearest
}

// GateManager 门控管理器
type GateManager struct {
	mu       sync.RWMutex
	gates    map[api.ResourceID]EventGate
	eventBus eventbus.EventBus
	subs     map[api.ResourceID]*eventbus.Subscription
}

// NewGateManager 创建门控管理器
func NewGateManager(eventBus eventbus.EventBus) *GateManager {
	return &GateManager{
		gates:    make(map[api.ResourceID]EventGate),
		eventBus: eventBus,
		subs:     make(map[api.ResourceID]*eventbus.Subscription),
	}
}

// CreateGate 为资源创建事件门控
func (gm *GateManager) CreateGate(resourceID api.ResourceID, required []api.RequiredEventSpec) EventGate {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	gate := NewEventGate(resourceID, required)
	gm.gates[resourceID] = gate

	// 订阅事件总线
	if len(required) > 0 {
		sub := gm.eventBus.Subscribe(func(event api.Event) bool {
			// 检查事件是否匹配任一需求
			for _, spec := range required {
				if spec.Fingerprint.Match(event) {
					return true
				}
			}
			return false
		})

		gm.subs[resourceID] = sub

		// 启动事件转发协程
		go gm.forwardEvents(resourceID, sub, gate)
	}

	return gate
}

// forwardEvents 转发事件到门控
func (gm *GateManager) forwardEvents(resourceID api.ResourceID, sub *eventbus.Subscription, gate EventGate) {
	for event := range sub.EventCh {
		gate.Notify(event)
	}
}

// GetGate 获取资源的门控
func (gm *GateManager) GetGate(resourceID api.ResourceID) EventGate {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	return gm.gates[resourceID]
}

// RemoveGate 移除门控
func (gm *GateManager) RemoveGate(resourceID api.ResourceID) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if gate, exists := gm.gates[resourceID]; exists {
		gate.Close()
		delete(gm.gates, resourceID)
	}

	if sub, exists := gm.subs[resourceID]; exists {
		gm.eventBus.Unsubscribe(sub)
		delete(gm.subs, resourceID)
	}
}

// Close 关闭所有门控
func (gm *GateManager) Close() {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	for _, gate := range gm.gates {
		gate.Close()
	}

	for _, sub := range gm.subs {
		gm.eventBus.Unsubscribe(sub)
	}

	gm.gates = make(map[api.ResourceID]EventGate)
	gm.subs = make(map[api.ResourceID]*eventbus.Subscription)
}
