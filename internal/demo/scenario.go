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
	"orchestrator/pkg/providers"
	"orchestrator/pkg/state"
)

// getDemoFactory 创建并配置好注册表的工厂
func getDemoFactory(bus eventbus.EventBus) *engine.ResourceFactory {
	factory := engine.NewResourceFactory(bus)

	// 注册 Tunnel 资源
	factory.Register("Tunnel", func(id string, cfg api.ResourceConfig, bus eventbus.EventBus) (api.Resource, error) {
		source, _ := cfg.Properties["source"].(string)
		target, _ := cfg.Properties["target"].(string)
		return providers.NewTunnelResource(id, source, target, cfg.Dependencies), nil
	})

	// 注册 Replica 资源
	factory.Register("Replica", func(id string, cfg api.ResourceConfig, bus eventbus.EventBus) (api.Resource, error) {
		data, _ := cfg.Properties["data"].(string)
		dc, _ := cfg.Properties["dc"].(string)
		return providers.NewReplicaResource(id, data, dc, cfg.Dependencies), nil
	})

	// 注册 Application 资源
	factory.Register("Application", func(id string, cfg api.ResourceConfig, bus eventbus.EventBus) (api.Resource, error) {
		name, _ := cfg.Properties["name"].(string)
		var dcs []string
		if rawDcs, ok := cfg.Properties["datacenters"].([]any); ok {
			for _, v := range rawDcs {
				if s, ok := v.(string); ok { dcs = append(dcs, s) }
			}
		}
		return providers.NewApplicationResource(id, name, dcs, cfg.Dependencies, bus), nil
	})

	return factory
}

// RunDemo 运行硬编码 Demo (已更新为引用 providers)
func RunDemo() error {
	fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
	fmt.Println("║     DAG-EDA 声明式编排引擎 Demo - 引擎插件分离架构               ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════╝")

	eventBus := eventbus.NewEventBus(256)
	defer eventBus.Close()
	stateStore := state.NewMemoryStateStore()
	graph := dag.NewGraph()

	tImg := providers.NewTunnelResource("T_Img", "DC3", "DC1/2", nil)
	rImg1 := providers.NewReplicaResource("R_Img_DC1", "Image-X", "DC1", []api.ResourceID{"T_Img"})
	tDat := providers.NewTunnelResource("T_Dat", "DC4", "DC1/2", nil)
	rDat1 := providers.NewReplicaResource("R_Dat_DC1", "Dataset-X", "DC1", []api.ResourceID{"T_Dat"})
	app1 := providers.NewApplicationResource("App_X_DC1", "App-X", []string{"DC1"}, []api.ResourceID{"R_Img_DC1", "R_Dat_DC1"}, eventBus)

	tMod := providers.NewTunnelResource("T_Mod", "DC1", "DC5", []api.ResourceID{"App_X_DC1"})
	tMod.SetRequiredEvents([]api.RequiredEventSpec{{
		Name: "AppFinished",
		Fingerprint: api.EventFingerprint{Type: api.EventTypeExternalSignal, Source: "App_X_DC1"},
		Required: true,
	}})
	rMod := providers.NewReplicaResource("R_Mod_DC5", "Model-X", "DC5", []api.ResourceID{"T_Mod"})
	rMod.SetRequiredEvents([]api.RequiredEventSpec{{
		Name: "ModelPath",
		Fingerprint: api.EventFingerprint{Type: api.EventTypeExternalSignal, Source: "App_X_DC1"},
		Required: true,
	}})

	resList := []api.Resource{tImg, rImg1, tDat, rDat1, app1, tMod, rMod}
	for _, res := range resList { graph.AddNode(res) }

	orchestrator := engine.NewOrchestrator("decoupled-workflow", graph, stateStore, eventBus, 5)
	for _, res := range resList { orchestrator.RegisterResource(res) }

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := orchestrator.Run(ctx); err != nil { return err }

	fmt.Println("\n" + strings.Repeat("═", 70))
	fmt.Println("🎉 Workflow Results Summary:")
	for id := range orchestrator.GetOutputs() { fmt.Printf("✅ %s Completed\n", id) }
	return nil
}

// RunJSONDemo 演示从 JSON 加载
func RunJSONDemo(jsonPath string) error {
	fmt.Println("\n" + strings.Repeat("=", 70))
	fmt.Printf("📂 Loading Decoupled Workflow from JSON: %s\n", jsonPath)
	fmt.Println(strings.Repeat("=", 70))

	eventBus := eventbus.NewEventBus(256)
	defer eventBus.Close()
	stateStore := state.NewMemoryStateStore()

	// 1. 创建通用加载器
	loader := engine.NewLoader(eventBus, stateStore)

	// 2. 配置工厂并注册 Demo 插件
	factory := getDemoFactory(eventBus)

	// 3. 执行加载并运行
	orchestrator, err := loader.LoadFromJSONWithFactory(jsonPath, factory)
	if err != nil { return err }

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if err := orchestrator.Run(ctx); err != nil { return err }

	fmt.Println("\n" + strings.Repeat("═", 70))
	fmt.Println("🎉 JSON Workflow Results Summary:")
	for id := range orchestrator.GetOutputs() { fmt.Printf("✅ %s Completed\n", id) }
	return nil
}
