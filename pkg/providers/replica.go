package providers

import (
	"context"
	"fmt"
	"time"

	"orchestrator/pkg/api"
)

type ReplicaResource struct {
	*api.ResourceBase
	DataName    string
	TargetDC    string
}

func NewReplicaResource(id, dataName, targetDC string, deps []api.ResourceID) *ReplicaResource {
	return &ReplicaResource{
		ResourceBase: &api.ResourceBase{
			ID:           api.ResourceID(id),
			Dependencies: deps,
			State:        api.StatePending,
		},
		DataName:    dataName,
		TargetDC:    targetDC,
	}
}

func (r *ReplicaResource) Provision(ctx context.Context, inputs api.ProvisionInputs) (api.ProvisionOutputs, error) {
	fmt.Printf("\n[Replica %s] 📦 Target: %s, Destination: %s\n", r.ID, r.DataName, r.TargetDC)

	path := "default/path"
	for _, payload := range inputs.EventPayloads {
		if p, ok := payload["model_path"].(string); ok {
			path = p
			fmt.Printf("[Replica %s] 🔗 Received dynamic path from event: %s\n", r.ID, path)
		}
	}

	time.Sleep(200 * time.Millisecond)
	fmt.Printf("[Replica %s] ✅ Data synchronized to %s\n", r.ID, r.TargetDC)

	r.State = api.StateSucceeded
	return api.ProvisionOutputs{
		Data: map[string]any{"status": "ready", "path": path},
		Events: []api.Event{api.NewEvent(api.EventTypeCustom, string(r.ID), map[string]any{"action": "replica_done"})},
	}, nil
}

func (r *ReplicaResource) Deprovision(ctx context.Context) error {
	return nil
}
