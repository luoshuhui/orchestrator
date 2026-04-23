package api

import "time"

// WorkflowConfig 定义整个工作流的 JSON 结构
type WorkflowConfig struct {
	WorkflowID string           `json:"workflow_id"`
	Resources  []ResourceConfig `json:"resources"`
}

// ResourceConfig 定义单个资源的配置
type ResourceConfig struct {
	ID             ResourceID          `json:"id"`
	Type           string              `json:"type"` // 例如: Tunnel, Replica, Application
	Dependencies   []ResourceID        `json:"dependencies,omitempty"`
	RequiredEvents []EventSpecConfig   `json:"required_events,omitempty"`
	Properties     map[string]interface{} `json:"properties"`
}

// EventSpecConfig 定义事件依赖的配置
type EventSpecConfig struct {
	Name       string        `json:"name"`
	EventType  EventType     `json:"event_type"`
	Source     string        `json:"source,omitempty"`
	Timeout    time.Duration `json:"timeout,omitempty"`
	PayloadKey string        `json:"payload_key,omitempty"`
	PayloadVal interface{}   `json:"payload_val,omitempty"`
}
