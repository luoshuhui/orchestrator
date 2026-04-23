package eventbus

import (
	"fmt"
	"sync"

	"orchestrator/pkg/api"
)

// Subscription 订阅句柄
type Subscription struct {
	ID      string
	EventCh chan api.Event
	Matcher EventMatcher
}

// EventMatcher 事件匹配器
type EventMatcher func(event api.Event) bool

// EventBus 事件总线接口
type EventBus interface {
	// Publish 发布事件
	Publish(event api.Event)

	// Subscribe 订阅事件
	// matcher 为 nil 表示订阅所有事件
	Subscribe(matcher EventMatcher) *Subscription

	// Unsubscribe 取消订阅
	Unsubscribe(sub *Subscription)

	// Close 关闭事件总线
	Close()
}

// eventBus 事件总线实现
type eventBus struct {
	mu            sync.RWMutex
	subscriptions map[string]*Subscription
	eventCh       chan api.Event
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

// NewEventBus 创建新的事件总线
func NewEventBus(bufferSize int) EventBus {
	bus := &eventBus{
		subscriptions: make(map[string]*Subscription),
		eventCh:       make(chan api.Event, bufferSize),
		stopCh:        make(chan struct{}),
	}

	// 启动事件分发协程
	bus.wg.Add(1)
	go bus.dispatchLoop()

	return bus
}

// Publish 发布事件
func (b *eventBus) Publish(event api.Event) {
	select {
	case b.eventCh <- event:
	case <-b.stopCh:
	}
}

// Subscribe 订阅事件
func (b *eventBus) Subscribe(matcher EventMatcher) *Subscription {
	b.mu.Lock()
	defer b.mu.Unlock()

	sub := &Subscription{
		ID:      generateSubscriptionID(),
		EventCh: make(chan api.Event, 64),
		Matcher: matcher,
	}

	b.subscriptions[sub.ID] = sub
	return sub
}

// Unsubscribe 取消订阅
func (b *eventBus) Unsubscribe(sub *Subscription) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.subscriptions[sub.ID]; exists {
		delete(b.subscriptions, sub.ID)
		close(sub.EventCh)
	}
}

// Close 关闭事件总线
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

// dispatchLoop 事件分发循环
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

// dispatch 分发事件到所有匹配的订阅者
func (b *eventBus) dispatch(event api.Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, sub := range b.subscriptions {
		// 如果没有匹配器或匹配成功，则发送事件
		if sub.Matcher == nil || sub.Matcher(event) {
			select {
			case sub.EventCh <- event:
			default:
				// 通道满，丢弃事件（避免阻塞）
			}
		}
	}
}

// generateSubscriptionID 生成订阅 ID
var subscriptionCounter int

func generateSubscriptionID() string {
	subscriptionCounter++
	return fmt.Sprintf("sub-%d", subscriptionCounter)
}

// MatchByFingerprint 创建基于事件指纹的匹配器
func MatchByFingerprint(fingerprint api.EventFingerprint) EventMatcher {
	return func(event api.Event) bool {
		return fingerprint.Match(event)
	}
}

// MatchByType 创建基于事件类型的匹配器
func MatchByType(eventType api.EventType) EventMatcher {
	return func(event api.Event) bool {
		return event.Type == eventType
	}
}

// MatchBySource 创建基于事件来源的匹配器
func MatchBySource(source string) EventMatcher {
	return func(event api.Event) bool {
		return event.Source == source
	}
}

// MatchAny 匹配所有事件
func MatchAny() EventMatcher {
	return func(event api.Event) bool {
		return true
	}
}
