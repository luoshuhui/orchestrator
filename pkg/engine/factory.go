package engine

import (
	"fmt"
	"orchestrator/pkg/api"
	"orchestrator/pkg/eventbus"
)

// ResourceCreator 是创建资源的工厂函数原型
type ResourceCreator func(id string, cfg api.ResourceConfig, bus eventbus.EventBus) (api.Resource, error)

// ResourceFactory 资源工厂
type ResourceFactory struct {
	eventBus eventbus.EventBus
	registry map[string]ResourceCreator
}

func NewResourceFactory(bus eventbus.EventBus) *ResourceFactory {
	return &ResourceFactory{
		eventBus: bus,
		registry: make(map[string]ResourceCreator),
	}
}

// Register 注册一种资源类型
func (f *ResourceFactory) Register(typeName string, creator ResourceCreator) {
	f.registry[typeName] = creator
}

// CreateResource 从配置创建资源对象
func (f *ResourceFactory) CreateResource(cfg api.ResourceConfig) (api.Resource, error) {
	creator, exists := f.registry[cfg.Type]
	if !exists {
		return nil, fmt.Errorf("unknown resource type: %s (did you forget to register it?)", cfg.Type)
	}

	res, err := creator(string(cfg.ID), cfg, f.eventBus)
	if err != nil {
		return nil, err
	}

	// 注入事件门控配置
	if len(cfg.RequiredEvents) > 0 {
		var specs []api.RequiredEventSpec
		for _, e := range cfg.RequiredEvents {
			spec := api.RequiredEventSpec{
				Name: e.Name,
				Fingerprint: api.EventFingerprint{
					Type:   e.EventType,
					Source: e.Source,
				},
				Required: true,
				Timeout:  e.Timeout,
			}
			if e.PayloadKey != "" {
				spec.Fingerprint.PayloadMatch = map[string]any{
					e.PayloadKey: e.PayloadVal,
				}
			}
			specs = append(specs, spec)
		}

		// 利用接口设置事件
		type eventSetter interface {
			SetRequiredEvents([]api.RequiredEventSpec)
		}
		if base, ok := res.(eventSetter); ok {
			base.SetRequiredEvents(specs)
		}
	}

	return res, nil
}
