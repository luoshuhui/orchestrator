package demo

import (
	"context"
	"fmt"
	"time"

	"orchestrator/pkg/api"
)

// ReplicaResource 代表数据中心中的数据副本对象
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
	fmt.Printf("\n[Replica %s] 📦 Starting replication: %s -> %s\n", r.ID, r.DataName, r.TargetDC)

	// 模拟拷贝过程
	time.Sleep(500 * time.Millisecond)
	fmt.Printf("[Replica %s] 🏎️  Transferring data blocks...\n", r.ID)
	time.Sleep(1 * time.Second)

	fmt.Printf("[Replica %s] ✅ Data synchronized to %s\n", r.ID, r.TargetDC)

	r.ResourceBase.State = api.StateSucceeded

	// 发送成功事件，用于触发上游隧道的清理
	successEvent := api.NewEvent(
		api.EventTypeCustom,
		string(r.ID),
		map[string]interface{}{
			"action": "replica_done",
			"replica": string(r.ID),
		},
	)

	return api.ProvisionOutputs{
		Data: map[string]interface{}{
			"replica_id": string(r.ID),
			"data":       r.DataName,
			"dc":         r.TargetDC,
			"status":     "ready",
		},
		Events: []api.Event{successEvent},
	}, nil
}

func (r *ReplicaResource) Deprovision(ctx context.Context) error {
	fmt.Printf("[Replica %s] 🧹 Deleting replica %s from %s\n", r.ID, r.DataName, r.TargetDC)
	r.ResourceBase.State = api.StatePending
	return nil
}
