package api

import (
	"context"
	"time"
)

// WorkflowState 工作流状态
type WorkflowState string

const (
	WorkflowStatePending   WorkflowState = "Pending"
	WorkflowStateRunning   WorkflowState = "Running"
	WorkflowStateSucceeded WorkflowState = "Succeeded"
	WorkflowStateFailed    WorkflowState = "Failed"
	WorkflowStateCancelled WorkflowState = "Cancelled"
)

// ResourceSnapshot 资源状态快照
// 用于持久化和恢复资源状态
type ResourceSnapshot struct {
	ID           ResourceID       `json:"id"`
	State        ResourceState    `json:"state"`
	Dependencies []ResourceID     `json:"dependencies"`
	Outputs      ProvisionOutputs `json:"outputs"`
	Error        string           `json:"error,omitempty"`
	StartedAt    *time.Time       `json:"startedAt,omitempty"`
	FinishedAt   *time.Time       `json:"finishedAt,omitempty"`
	// 事件门控状态
	PendingEvents []RequiredEventSpec `json:"pendingEvents,omitempty"`
	ReceivedEvent []Event             `json:"receivedEvents,omitempty"`
}

// WorkflowSnapshot 工作流状态快照
// 用于持久化和恢复整个工作流的状态
type WorkflowSnapshot struct {
	WorkflowID string        `json:"workflowId"`
	Name       string        `json:"name"`
	State      WorkflowState `json:"state"`
	// 资源快照映射
	Resources map[ResourceID]*ResourceSnapshot `json:"resources"`
	// 执行层级（拓扑排序结果）
	ExecutionLayers [][]ResourceID `json:"executionLayers"`
	// 当前执行层级
	CurrentLayer int `json:"currentLayer"`
	// 创建时间
	CreatedAt time.Time `json:"createdAt"`
	// 更新时间
	UpdatedAt time.Time `json:"updatedAt"`
	// 错误信息
	Error string `json:"error,omitempty"`
}

// StateStore 状态持久化接口
// 确保长任务的上下文可以持久化，防止引擎重启导致逻辑断档
type StateStore interface {
	// SaveWorkflow 保存工作流快照
	SaveWorkflow(ctx context.Context, snapshot WorkflowSnapshot) error

	// LoadWorkflow 加载工作流快照
	LoadWorkflow(ctx context.Context, workflowID string) (*WorkflowSnapshot, error)

	// SaveResource 保存资源快照
	SaveResource(ctx context.Context, workflowID string, snapshot ResourceSnapshot) error

	// LoadResource 加载资源快照
	LoadResource(ctx context.Context, workflowID string, resourceID ResourceID) (*ResourceSnapshot, error)

	// ListPendingWorkflows 列出所有未完成的工作流
	// 用于引擎重启后恢复执行
	ListPendingWorkflows(ctx context.Context) ([]WorkflowSnapshot, error)

	// DeleteWorkflow 删除工作流快照
	DeleteWorkflow(ctx context.Context, workflowID string) error
}

// NewWorkflowSnapshot 创建新的工作流快照
func NewWorkflowSnapshot(workflowID, name string) *WorkflowSnapshot {
	return &WorkflowSnapshot{
		WorkflowID: workflowID,
		Name:       name,
		State:      WorkflowStatePending,
		Resources:  make(map[ResourceID]*ResourceSnapshot),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

// NewResourceSnapshot 创建新的资源快照
func NewResourceSnapshot(id ResourceID) *ResourceSnapshot {
	return &ResourceSnapshot{
		ID:    id,
		State: StatePending,
	}
}
