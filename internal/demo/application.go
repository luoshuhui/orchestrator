package demo

import (
	"context"
	"fmt"
	"time"

	"orchestrator/pkg/api"
	"orchestrator/pkg/eventbus"
)

// ApplicationResource 应用部署资源
type ApplicationResource struct {
	*api.ResourceBase
	Name        string
	Datacenters []string
	EventBus    eventbus.EventBus
}

// NewApplicationResource 创建应用资源
func NewApplicationResource(id, name string, datacenters []string, deps []api.ResourceID, bus eventbus.EventBus) *ApplicationResource {
	return &ApplicationResource{
		ResourceBase: &api.ResourceBase{
			ID:           api.ResourceID(id),
			Dependencies: deps,
			State:        api.StatePending,
		},
		Name:        name,
		Datacenters: datacenters,
		EventBus:    bus,
	}
}

// WithRequiredEvents 设置所需事件
func (a *ApplicationResource) WithRequiredEvents(events []api.RequiredEventSpec) *ApplicationResource {
	a.ResourceBase.Events = events
	return a
}

// Provision 部署应用
func (a *ApplicationResource) Provision(ctx context.Context, inputs api.ProvisionInputs) (api.ProvisionOutputs, error) {
	fmt.Printf("\n[App %s] 🚀 Deploying application '%s' to datacenters: %v\n", a.ID, a.Name, a.Datacenters)

	// 模拟在每个数据中心部署
	for _, dc := range a.Datacenters {
		select {
		case <-ctx.Done():
			return api.ProvisionOutputs{}, ctx.Err()
		default:
			fmt.Printf("[App %s] 🏗️  Deploying to %s...\n", a.ID, dc)
			time.Sleep(500 * time.Millisecond)
			fmt.Printf("[App %s] ✅ Deployed to %s\n", a.ID, dc)
		}
	}

	// 模拟应用运行
	fmt.Printf("[App %s] 🔄 Application running...\n", a.ID)
	time.Sleep(800 * time.Millisecond)

	fmt.Printf("[App %s] 🏁 Application finished execution\n", a.ID)

	a.ResourceBase.State = api.StateSucceeded

	// 发送完成信号
	finishEvent := api.NewEvent(
		api.EventTypeExternalSignal,
		string(a.ID),
		map[string]interface{}{
			"signal":      "m-finish",
			"application": a.Name,
			"timestamp":   time.Now().Format(time.RFC3339),
		},
	)

	fmt.Printf("[App %s] 📡 Sending finish signal: %s\n", a.ID, "m-finish")

	return api.ProvisionOutputs{
		Data: map[string]interface{}{
			"app_id":      string(a.ID),
			"name":        a.Name,
			"datacenters": a.Datacenters,
			"status":      "finished",
		},
		Events: []api.Event{finishEvent},
	}, nil
}

// Deprovision 销毁应用
func (a *ApplicationResource) Deprovision(ctx context.Context) error {
	fmt.Printf("[App %s] 🧹 Decommissioning application '%s'\n", a.ID, a.Name)
	a.ResourceBase.State = api.StatePending
	return nil
}
