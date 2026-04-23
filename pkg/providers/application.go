package providers

import (
	"context"
	"fmt"
	"time"

	"orchestrator/pkg/api"
	"orchestrator/pkg/eventbus"
)

type ApplicationResource struct {
	*api.ResourceBase
	Name        string
	Datacenters []string
	EventBus    eventbus.EventBus
}

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

func (a *ApplicationResource) Provision(ctx context.Context, inputs api.ProvisionInputs) (api.ProvisionOutputs, error) {
	fmt.Printf("\n[App %s] 🏗️  Step 1: Deploying %s to %v\n", a.ID, a.Name, a.Datacenters)
	time.Sleep(1 * time.Second)

	fmt.Printf("[App %s] 🔄 Step 2: Application is now RUNNING and processing data...\n", a.ID)
	time.Sleep(2 * time.Second)

	fmt.Printf("[App %s] 🏁 Step 3: Business logic COMPLETED. Model generated.\n", a.ID)
	a.State = api.StateSucceeded

	finishEvent := api.NewEvent(
		api.EventTypeExternalSignal,
		string(a.ID),
		map[string]any{
			"status":     "finished",
			"model_path": "/tmp/model.bin",
		},
	)

	return api.ProvisionOutputs{
		Data:   map[string]any{"status": "finished"},
		Events: []api.Event{finishEvent},
	}, nil
}

func (a *ApplicationResource) Deprovision(ctx context.Context) error {
	return nil
}
