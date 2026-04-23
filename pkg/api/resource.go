package api

import (
	"context"
)

// ResourceID 资源唯一标识
type ResourceID string

// ResourceState 资源状态枚举
type ResourceState string

const (
	StatePending   ResourceState = "Pending"   // 等待依赖
	StateBlocked   ResourceState = "Blocked"   // 被事件门控阻塞
	StateRunning   ResourceState = "Running"   // 正在执行
	StateSucceeded ResourceState = "Succeeded" // 执行成功
	StateFailed    ResourceState = "Failed"    // 执行失败
	StateSkipped   ResourceState = "Skipped"   // 跳过执行
)

// ProvisionInputs Provision 操作的输入参数
type ProvisionInputs struct {
	UpstreamOutputs map[ResourceID]ProvisionOutputs
	EventPayloads   map[string]map[string]any
	Context         map[string]any
}

// ProvisionOutputs Provision 操作的输出
type ProvisionOutputs struct {
	Data   map[string]any
	Events []Event
}

// Resource 核心资源接口
type Resource interface {
	Identity() ResourceID
	GetDependencies() []ResourceID
	RequiredEvents() []RequiredEventSpec
	Provision(ctx context.Context, inputs ProvisionInputs) (ProvisionOutputs, error)
	Deprovision(ctx context.Context) error
	ActualState() ResourceState
}

// ResourceBase 提供 Resource 接口的默认实现
type ResourceBase struct {
	ID           ResourceID
	Dependencies []ResourceID
	Events       []RequiredEventSpec
	State        ResourceState
}

func (r *ResourceBase) Identity() ResourceID {
	return r.ID
}

func (r *ResourceBase) GetDependencies() []ResourceID {
	return r.Dependencies
}

func (r *ResourceBase) RequiredEvents() []RequiredEventSpec {
	return r.Events
}

func (r *ResourceBase) ActualState() ResourceState {
	return r.State
}

func (r *ResourceBase) SetState(state ResourceState) {
	r.State = state
}

func (r *ResourceBase) SetRequiredEvents(events []RequiredEventSpec) {
	r.Events = events
}
