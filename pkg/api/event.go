package api

import (
	"time"
)

// EventType 事件类型枚举
type EventType string

const (
	EventTypeProvisionSuccess EventType = "ProvisionSuccess"
	EventTypeProvisionFailure EventType = "ProvisionFailure"
	EventTypeDeprovisionDone  EventType = "DeprovisionDone"
	EventTypeExternalSignal   EventType = "ExternalSignal"
	EventTypeTimeout          EventType = "Timeout"
	EventTypeCustom           EventType = "Custom"
)

// Event 表示系统中传递的事件
type Event struct {
	ID        string                 // 事件唯一标识
	Type      EventType              // 事件类型
	Source    string                 // 事件来源
	Timestamp time.Time              // 事件发生时间
	Payload   map[string]interface{} // 事件负载数据
}

// EventFingerprint 事件指纹，用于精确匹配所需事件
// 设计思路：通过组合事件特征形成唯一标识，支持模糊匹配和精确匹配
type EventFingerprint struct {
	Type         EventType              // 必须匹配的事件类型
	Source       string                 // 可选：事件来源过滤
	Labels       map[string]string      // 标签选择器
	PayloadMatch map[string]interface{} // 可选：负载内容匹配
}

// Match 判断事件是否匹配指纹
func (ef *EventFingerprint) Match(event Event) bool {
	// 类型必须匹配
	if event.Type != ef.Type {
		return false
	}

	// 来源过滤（如果指定）
	if ef.Source != "" && event.Source != ef.Source {
		return false
	}

	// 负载匹配
	for k, v := range ef.PayloadMatch {
		if payloadVal, ok := event.Payload[k]; !ok || payloadVal != v {
			return false
		}
	}

	return true
}

// RequiredEventSpec 描述资源所需事件的完整规格
type RequiredEventSpec struct {
	Fingerprint EventFingerprint // 事件指纹
	Timeout     time.Duration    // 等待超时（0 表示无超时）
	Required    bool             // 是否必须（可选事件 vs 必须事件）
	Name        string           // 事件名称，用于日志和调试
}

// NewEvent 创建新事件
func NewEvent(eventType EventType, source string, payload map[string]interface{}) Event {
	return Event{
		ID:        generateEventID(),
		Type:      eventType,
		Source:    source,
		Timestamp: time.Now(),
		Payload:   payload,
	}
}

// generateEventID 生成事件 ID
func generateEventID() string {
	return time.Now().Format("20060102150405.999999999")
}
