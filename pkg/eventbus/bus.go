package eventbus

import (
	"fmt"
	"sync"

	"orchestrator/pkg/api"
)

type Subscription struct {
	ID      string
	EventCh chan api.Event
	Matcher EventMatcher
}

type EventMatcher func(event api.Event) bool

type EventBus interface {
	Publish(event api.Event)
	Subscribe(matcher EventMatcher) *Subscription
	Unsubscribe(sub *Subscription)
	GetHistory() []api.Event // 获取历史事件
	Close()
}

type eventBus struct {
	mu            sync.RWMutex
	subscriptions map[string]*Subscription
	eventCh       chan api.Event
	history       []api.Event // 增加历史记录
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

func NewEventBus(bufferSize int) EventBus {
	bus := &eventBus{
		subscriptions: make(map[string]*Subscription),
		eventCh:       make(chan api.Event, bufferSize),
		history:       make([]api.Event, 0),
		stopCh:        make(chan struct{}),
	}
	bus.wg.Add(1)
	go bus.dispatchLoop()
	return bus
}

func (b *eventBus) Publish(event api.Event) {
	b.mu.Lock()
	b.history = append(b.history, event) // 存入历史
	b.mu.Unlock()

	select {
	case b.eventCh <- event:
	case <-b.stopCh:
	}
}

func (b *eventBus) GetHistory() []api.Event {
	b.mu.RLock()
	defer b.mu.RUnlock()
	res := make([]api.Event, len(b.history))
	copy(res, b.history)
	return res
}

func (b *eventBus) Subscribe(matcher EventMatcher) *Subscription {
	b.mu.Lock()
	defer b.mu.Unlock()

	sub := &Subscription{
		ID:      generateSubscriptionID(),
		EventCh: make(chan api.Event, 64),
		Matcher: matcher,
	}

	// 订阅时，立即将历史中匹配的事件补发给新订阅者
	for _, ev := range b.history {
		if matcher == nil || matcher(ev) {
			sub.EventCh <- ev
		}
	}

	b.subscriptions[sub.ID] = sub
	return sub
}

func (b *eventBus) Unsubscribe(sub *Subscription) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.subscriptions[sub.ID]; exists {
		delete(b.subscriptions, sub.ID)
		close(sub.EventCh)
	}
}

func (b *eventBus) Close() {
	close(b.stopCh)
	b.wg.Wait()
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, sub := range b.subscriptions {
		close(sub.EventCh)
	}
	b.subscriptions = make(map[string]*Subscription)
}

func (b *eventBus) dispatchLoop() {
	defer b.wg.Done()
	for {
		select {
		case event := <-b.eventCh:
			b.dispatch(event)
		case <-b.stopCh:
			return
		}
	}
}

func (b *eventBus) dispatch(event api.Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, sub := range b.subscriptions {
		if sub.Matcher == nil || sub.Matcher(event) {
			select {
			case sub.EventCh <- event:
			default:
			}
		}
	}
}

var subscriptionCounter int
var counterMu sync.Mutex

func generateSubscriptionID() string {
	counterMu.Lock()
	defer counterMu.Unlock()
	subscriptionCounter++
	return fmt.Sprintf("sub-%d", subscriptionCounter)
}

func MatchByFingerprint(fingerprint api.EventFingerprint) EventMatcher {
	return func(event api.Event) bool { return fingerprint.Match(event) }
}

func MatchByType(eventType api.EventType) EventMatcher {
	return func(event api.Event) bool { return event.Type == eventType }
}

func MatchAny() EventMatcher {
	return func(event api.Event) bool { return true }
}
