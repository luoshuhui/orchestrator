package demo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"orchestrator/pkg/api"
	"orchestrator/pkg/dag"
	"orchestrator/pkg/engine"
	"orchestrator/pkg/eventbus"
	"orchestrator/pkg/state"
)

// RunDemo 运行跨中心部署与模型回收的复杂 Demo 场景
func RunDemo() error {
	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║     DAG-EDA 声明式编排引擎 Demo - 跨中心同步与模型回收            ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")

	eventBus := eventbus.NewEventBus(256)
	defer eventBus.Close()
	stateStore := state.NewMemoryStateStore()
	graph := dag.NewGraph()

	// 1. 镜像隧道与拷贝 (DC3 -> DC1, DC2)
	tImg := NewTunnelResource("T_Img", "DC3", "DC1/2", nil)
	rImg1 := NewReplicaResource("R_Img_DC1", "Image-X", "DC1", []api.ResourceID{"T_Img"})
	rImg2 := NewReplicaResource("R_Img_DC2", "Image-X", "DC2", []api.ResourceID{"T_Img"})

	// 2. 数据集隧道与拷贝 (DC4 -> DC1, DC2)
	tDat := NewTunnelResource("T_Dat", "DC4", "DC1/2", nil)
	rDat1 := NewReplicaResource("R_Dat_DC1", "Dataset-X", "DC1", []api.ResourceID{"T_Dat"})
	rDat2 := NewReplicaResource("R_Dat_DC2", "Dataset-X", "DC2", []api.ResourceID{"T_Dat"})

	// 3. 应用部署 (依赖镜像和数据就绪)
	app1 := NewApplicationResource("App_X_DC1", "App-X", []string{"DC1"}, []api.ResourceID{"R_Img_DC1", "R_Dat_DC1"}, eventBus)
	app2 := NewApplicationResource("App_X_DC2", "App-X", []string{"DC2"}, []api.ResourceID{"R_Img_DC2", "R_Dat_DC2"}, eventBus)

	// 4. 模型回收隧道与副本 (DC1 -> DC5)
	// T_Mod 依赖 App_X_DC1 运行结束信号
	tMod := NewTunnelResource("T_Mod", "DC1", "DC5", []api.ResourceID{"App_X_DC1"}).
		WithRequiredEvents([]api.RequiredEventSpec{
			{
				Name: "AppFinished",
				Fingerprint: api.EventFingerprint{
					Type:   api.EventTypeExternalSignal,
					Source: "App_X_DC1",
				},
				Required: true,
			},
		})

	rMod := NewReplicaResource("R_Mod_DC5", "Model-X", "DC5", []api.ResourceID{"T_Mod"})

	// 添加到图
	resources := []api.Resource{tImg, rImg1, rImg2, tDat, rDat1, rDat2, app1, app2, tMod, rMod}
	for _, res := range resources {
		graph.AddNode(res)
	}

	orchestrator := engine.NewOrchestrator("complex-workflow-001", graph, stateStore, eventBus, 5)
	for _, res := range resources {
		orchestrator.RegisterResource(res)
	}

	fmt.Println("\n🚀 Starting Complex Workflow Execution...")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if err := orchestrator.Run(ctx); err != nil {
		return err
	}

	fmt.Println("\n" + strings.Repeat("═", 70))
	fmt.Println("🎉 Workflow Results Summary:")
	outputs := orchestrator.GetOutputs()
	for id := range outputs {
		fmt.Printf("✅ %s Completed\n", id)
	}

	return nil
}
