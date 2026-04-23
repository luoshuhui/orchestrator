package state

import (
	"context"
	"sync"

	"orchestrator/pkg/api"
)

// MemoryStateStore 内存状态存储
type MemoryStateStore struct {
	mu        sync.RWMutex
	workflows map[string]*api.WorkflowSnapshot
	resources map[string]map[api.ResourceID]*api.ResourceSnapshot
}

// NewMemoryStateStore 创建内存状态存储
func NewMemoryStateStore() *MemoryStateStore {
	return &MemoryStateStore{
		workflows: make(map[string]*api.WorkflowSnapshot),
		resources: make(map[string]map[api.ResourceID]*api.ResourceSnapshot),
	}
}

// SaveWorkflow 保存工作流快照
func (s *MemoryStateStore) SaveWorkflow(ctx context.Context, snapshot api.WorkflowSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.workflows[snapshot.WorkflowID] = &snapshot
	return nil
}

// LoadWorkflow 加载工作流快照
func (s *MemoryStateStore) LoadWorkflow(ctx context.Context, workflowID string) (*api.WorkflowSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.workflows[workflowID], nil
}

// SaveResource 保存资源快照
func (s *MemoryStateStore) SaveResource(ctx context.Context, workflowID string, snapshot api.ResourceSnapshot) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.resources[workflowID] == nil {
		s.resources[workflowID] = make(map[api.ResourceID]*api.ResourceSnapshot)
	}
	s.resources[workflowID][snapshot.ID] = &snapshot
	return nil
}

// LoadResource 加载资源快照
func (s *MemoryStateStore) LoadResource(ctx context.Context, workflowID string, resourceID api.ResourceID) (*api.ResourceSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if wf, ok := s.resources[workflowID]; ok {
		return wf[resourceID], nil
	}
	return nil, nil
}

// ListPendingWorkflows 列出所有未完成的工作流
func (s *MemoryStateStore) ListPendingWorkflows(ctx context.Context) ([]api.WorkflowSnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var pending []api.WorkflowSnapshot
	for _, wf := range s.workflows {
		if wf.State == api.WorkflowStateRunning || wf.State == api.WorkflowStatePending {
			pending = append(pending, *wf)
		}
	}
	return pending, nil
}

// DeleteWorkflow 删除工作流快照
func (s *MemoryStateStore) DeleteWorkflow(ctx context.Context, workflowID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.workflows, workflowID)
	delete(s.resources, workflowID)
	return nil
}
