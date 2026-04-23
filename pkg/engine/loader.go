package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"orchestrator/pkg/api"
	"orchestrator/pkg/dag"
	"orchestrator/pkg/eventbus"
)

// Loader 负责加载 JSON 配置并初始化编排引擎
type Loader struct {
	eventBus   eventbus.EventBus
	stateStore api.StateStore
}

func NewLoader(bus eventbus.EventBus, store api.StateStore) *Loader {
	return &Loader{
		eventBus:   bus,
		stateStore: store,
	}
}

// LoadFromJSONWithFactory 使用提供的工厂加载配置
func (l *Loader) LoadFromJSONWithFactory(filePath string, factory *ResourceFactory) (*Orchestrator, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var config api.WorkflowConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	graph := dag.NewGraph()
	orchestrator := NewOrchestrator(
		config.WorkflowID,
		graph,
		l.stateStore,
		l.eventBus,
		10,
	)

	for _, resCfg := range config.Resources {
		res, err := factory.CreateResource(resCfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create resource %s: %w", resCfg.ID, err)
		}

		if err := graph.AddNode(res); err != nil {
			return nil, fmt.Errorf("failed to add node %s to graph: %w", resCfg.ID, err)
		}

		orchestrator.RegisterResource(res)
	}

	return orchestrator, nil
}

// LoadFromJSON 已废弃，请使用 LoadFromJSONWithFactory
func (l *Loader) LoadFromJSON(filePath string) (*Orchestrator, error) {
	return nil, fmt.Errorf("LoadFromJSON is deprecated, please use LoadFromJSONWithFactory and provide a configured factory")
}
