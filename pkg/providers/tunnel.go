package providers

import (
	"context"
	"fmt"
	"time"

	"orchestrator/pkg/api"
)

type TunnelResource struct {
	*api.ResourceBase
	Source      string
	Destination string
}

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

func (t *TunnelResource) Provision(ctx context.Context, inputs api.ProvisionInputs) (api.ProvisionOutputs, error) {
	fmt.Printf("\n[Tunnel %s] 🚀 Creating tunnel %s -> %s\n", t.ID, t.Source, t.Destination)
	time.Sleep(500 * time.Millisecond)
	fmt.Printf("[Tunnel %s] ✅ Migration completed successfully\n", t.ID)
	t.State = api.StateSucceeded
	return api.ProvisionOutputs{
		Data: map[string]any{"tunnel_id": string(t.ID), "status": "active"},
	}, nil
}

func (t *TunnelResource) Deprovision(ctx context.Context) error {
	fmt.Printf("[Tunnel %s] 🧹 Cleaning up tunnel\n", t.ID)
	return nil
}
