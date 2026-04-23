package demo

import (
	"context"
	"fmt"
	"time"

	"orchestrator/pkg/api"
)

// TunnelResource 数据迁移隧道资源
type TunnelResource struct {
	*api.ResourceBase
	Source      string
	Destination string
}

// NewTunnelResource 创建隧道资源
func NewTunnelResource(id, source, dest string, deps []api.ResourceID) *TunnelResource {
	return &TunnelResource{
		ResourceBase: &api.ResourceBase{
			ID:           api.ResourceID(id),
			Dependencies: deps,
			State:        api.StatePending,
		},
		Source:      source,
		Destination: dest,
	}
}

// WithRequiredEvents 设置所需事件
func (t *TunnelResource) WithRequiredEvents(events []api.RequiredEventSpec) *TunnelResource {
	t.ResourceBase.Events = events
	return t
}

// Provision 执行隧道创建和数据迁移
func (t *TunnelResource) Provision(ctx context.Context, inputs api.ProvisionInputs) (api.ProvisionOutputs, error) {
	fmt.Printf("\n[Tunnel %s] 🚀 Creating tunnel %s -> %s\n", t.ID, t.Source, t.Destination)

	// 模拟创建隧道
	fmt.Printf("[Tunnel %s] 🔗 Establishing connection...\n", t.ID)
	time.Sleep(300 * time.Millisecond)

	// 模拟数据迁移
	for i := 0; i < 5; i++ {
		select {
		case <-ctx.Done():
			return api.ProvisionOutputs{}, ctx.Err()
		case <-time.After(400 * time.Millisecond):
			progress := (i + 1) * 20
			fmt.Printf("[Tunnel %s] 📦 Migrating data... %d%%\n", t.ID, progress)
		}
	}

	// 模拟删除隧道
	fmt.Printf("[Tunnel %s] 🗑️  Removing tunnel after migration...\n", t.ID)
	time.Sleep(200 * time.Millisecond)

	fmt.Printf("[Tunnel %s] ✅ Migration completed successfully\n", t.ID)

	t.ResourceBase.State = api.StateSucceeded
	return api.ProvisionOutputs{
		Data: map[string]interface{}{
			"tunnel_id":   string(t.ID),
			"source":      t.Source,
			"destination": t.Destination,
			"status":      "migrated",
			"timestamp":   time.Now().Format(time.RFC3339),
		},
	}, nil
}

// Deprovision 销毁隧道
func (t *TunnelResource) Deprovision(ctx context.Context) error {
	fmt.Printf("[Tunnel %s] 🧹 Cleaning up tunnel %s -> %s\n", t.ID, t.Source, t.Destination)
	t.ResourceBase.State = api.StatePending
	return nil
}
