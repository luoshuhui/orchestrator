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
// 支持数据流绑定：上游节点的 Output 以及 Event.Payload 动态注入
type ProvisionInputs struct {
	// UpstreamOutputs 上游节点的输出，key 为 ResourceID
	UpstreamOutputs map[ResourceID]ProvisionOutputs

	// EventPayloads 匹配到的事件负载，key 为事件名称
	EventPayloads map[string]map[string]interface{}

	// Context 自定义上下文数据
	Context map[string]interface{}
}

// ProvisionOutputs Provision 操作的输出
type ProvisionOutputs struct {
	// Data 输出数据，可供下游节点使用
	Data map[string]interface{}

	// Events 执行过程中产生的事件（如完成信号）
	Events []Event
}

// Resource 核心资源接口
// 所有编排资源必须实现此接口
type Resource interface {
	// Identity 返回资源唯一标识
	Identity() ResourceID

	// GetDependencies 返回静态拓扑依赖
	// 这些依赖必须在当前资源执行前完成
	GetDependencies() []ResourceID

	// RequiredEvents 返回该资源激活所需的事件指纹
	// 返回空切片表示无事件依赖
	RequiredEvents() []RequiredEventSpec

	// Provision 执行资源的创建/配置操作
	// inputs 包含上游输出和事件负载
	// 返回输出数据和产生的事件
	Provision(ctx context.Context, inputs ProvisionInputs) (ProvisionOutputs, error)

	// Deprovision 执行资源的销毁/清理操作
	Deprovision(ctx context.Context) error

	// ActualState 返回资源当前实际状态
	ActualState() ResourceState
}

// ResourceBase 提供 Resource 接口的默认实现
// 可嵌入其他资源结构体以减少重复代码
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

// SetState 设置资源状态
func (r *ResourceBase) SetState(state ResourceState) {
	r.State = state
}
