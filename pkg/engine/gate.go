package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"orchestrator/pkg/api"
	"orchestrator/pkg/eventbus"
)

type GateState string

const (
	GateStateOpen     GateState = "Open"
	GateStateClosed   GateState = "Closed"
	GateStateTimedOut GateState = "TimedOut"
)

type GateStatus struct {
	ResourceID     api.ResourceID
	RequiredEvents []api.RequiredEventSpec
	ReceivedEvents []api.Event
	PendingCount   int
	State          GateState
}

type EventGate interface {
	Wait(ctx context.Context) ([]api.Event, error)
	Notify(event api.Event) bool
	Status() GateStatus
	Close()
}

type eventGate struct {
	resourceID     api.ResourceID
	requiredEvents []api.RequiredEventSpec
	receivedEvents []api.Event
	eventCh        chan api.Event
	doneCh         chan struct{}
	mu             sync.RWMutex
	closed         bool
}

func NewEventGate(resourceID api.ResourceID, required []api.RequiredEventSpec) EventGate {
	return &eventGate{
		resourceID:     resourceID,
		requiredEvents: required,
		eventCh:        make(chan api.Event, 128), // 增大缓冲区
		doneCh:         make(chan struct{}),
	}
}

func (g *eventGate) Wait(ctx context.Context) ([]api.Event, error) {
	for {
		status := g.Status()
		if status.State == GateStateOpen {
			return status.ReceivedEvents, nil
		}

		select {
		case <-g.eventCh:
			// 每次从 channel 醒来都重新检查一次 Status
			continue
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-g.doneCh:
			return nil, fmt.Errorf("gate closed")
		case <-time.After(1 * time.Second):
			// 强制心跳检查
			continue
		}
	}
}

func (g *eventGate) Notify(event api.Event) bool {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.closed {
		return false
	}

	// 检查是否匹配任何所需的事件
	matched := false
	for _, spec := range g.requiredEvents {
		if spec.Fingerprint.Match(event) {
			matched = true
			break
		}
	}

	if matched {
		g.receivedEvents = append(g.receivedEvents, event)
		// 唤醒 Wait 协程
		select {
		case g.eventCh <- event:
		default:
		}
		return true
	}

	return false
}

func (g *eventGate) Status() GateStatus {
	g.mu.RLock()
	defer g.mu.RUnlock()

	matchedSpecs := make(map[string]bool)
	for _, ev := range g.receivedEvents {
		for _, spec := range g.requiredEvents {
			if spec.Fingerprint.Match(ev) {
				matchedSpecs[spec.Name] = true
			}
		}
	}

	pendingCount := 0
	for _, spec := range g.requiredEvents {
		if spec.Required && !matchedSpecs[spec.Name] {
			pendingCount++
		}
	}

	state := GateStateClosed
	if pendingCount == 0 {
		state = GateStateOpen
	}

	return GateStatus{
		ResourceID:     g.resourceID,
		RequiredEvents: g.requiredEvents,
		ReceivedEvents: g.receivedEvents,
		PendingCount:   pendingCount,
		State:          state,
	}
}

func (g *eventGate) Close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.closed {
		g.closed = true
		close(g.doneCh)
	}
}

type GateManager struct {
	mu       sync.RWMutex
	gates    map[api.ResourceID]EventGate
	eventBus eventbus.EventBus
	subs     map[api.ResourceID]*eventbus.Subscription
}

func NewGateManager(eventBus eventbus.EventBus) *GateManager {
	return &GateManager{
		gates:    make(map[api.ResourceID]EventGate),
		eventBus: eventBus,
		subs:     make(map[api.ResourceID]*eventbus.Subscription),
	}
}

func (gm *GateManager) CreateGate(resourceID api.ResourceID, required []api.RequiredEventSpec) EventGate {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	gate := NewEventGate(resourceID, required)
	gm.gates[resourceID] = gate

	if len(required) > 0 {
		sub := gm.eventBus.Subscribe(func(event api.Event) bool {
			for _, spec := range required {
				if spec.Fingerprint.Match(event) {
					return true
				}
			}
			return false
		})

		gm.subs[resourceID] = sub
		go func() {
			for event := range sub.EventCh {
				gate.Notify(event)
			}
		}()
	}

	return gate
}

func (gm *GateManager) GetGate(resourceID api.ResourceID) EventGate {
	gm.mu.RLock()
	defer gm.mu.RUnlock()
	return gm.gates[resourceID]
}

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

func (gm *GateManager) Close() {
	gm.mu.Lock()
	defer gm.mu.Unlock()
	for _, gate := range gm.gates { gate.Close() }
	for _, sub := range gm.subs { gm.eventBus.Unsubscribe(sub) }
	gm.gates = make(map[api.ResourceID]EventGate)
	gm.subs = make(map[api.ResourceID]*eventbus.Subscription)
}
