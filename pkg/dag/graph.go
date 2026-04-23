package dag

import (
	"fmt"

	"orchestrator/pkg/api"
)

// Node DAG 节点
type Node struct {
	ID           api.ResourceID   // 节点 ID
	Resource     api.Resource     // 资源实例
	Dependencies []api.ResourceID // 依赖的节点 ID
	Dependents   []api.ResourceID // 依赖此节点的节点 ID
	// 事件依赖
	RequiredEvents []api.RequiredEventSpec
}

// Graph DAG 图结构
type Graph struct {
	Nodes         map[api.ResourceID]*Node // 所有节点
	AdjacencyList map[api.ResourceID][]api.ResourceID // 邻接表（依赖关系）
	ReverseAdj    map[api.ResourceID][]api.ResourceID // 反向邻接表（被依赖关系）
}

// NewGraph 创建新的 DAG 图
func NewGraph() *Graph {
	return &Graph{
		Nodes:         make(map[api.ResourceID]*Node),
		AdjacencyList: make(map[api.ResourceID][]api.ResourceID),
		ReverseAdj:    make(map[api.ResourceID][]api.ResourceID),
	}
}

// AddNode 添加节点
func (g *Graph) AddNode(resource api.Resource) error {
	id := resource.Identity()
	if _, exists := g.Nodes[id]; exists {
		return fmt.Errorf("node %s already exists", id)
	}

	node := &Node{
		ID:             id,
		Resource:       resource,
		Dependencies:   resource.GetDependencies(),
		Dependents:     []api.ResourceID{},
		RequiredEvents: resource.RequiredEvents(),
	}

	g.Nodes[id] = node
	g.AdjacencyList[id] = node.Dependencies
	g.ReverseAdj[id] = []api.ResourceID{}

	// 建立反向依赖关系
	for _, dep := range node.Dependencies {
		if _, exists := g.ReverseAdj[dep]; !exists {
			g.ReverseAdj[dep] = []api.ResourceID{}
		}
		g.ReverseAdj[dep] = append(g.ReverseAdj[dep], id)
	}

	return nil
}

// GetNode 获取节点
func (g *Graph) GetNode(id api.ResourceID) (*Node, bool) {
	node, exists := g.Nodes[id]
	return node, exists
}

// GetReadyNodes 获取所有就绪的节点（依赖已满足）
func (g *Graph) GetReadyNodes(completed map[api.ResourceID]bool) []api.ResourceID {
	var ready []api.ResourceID
	for id, node := range g.Nodes {
		if completed[id] {
			continue
		}
		if g.dependenciesSatisfied(node, completed) {
			ready = append(ready, id)
		}
	}
	return ready
}

// dependenciesSatisfied 检查节点的依赖是否已满足
func (g *Graph) dependenciesSatisfied(node *Node, completed map[api.ResourceID]bool) bool {
	for _, dep := range node.Dependencies {
		if !completed[dep] {
			return false
		}
	}
	return true
}

// GetDependents 获取依赖指定节点的所有节点
func (g *Graph) GetDependents(id api.ResourceID) []api.ResourceID {
	return g.ReverseAdj[id]
}

// GetDependencies 获取指定节点的所有依赖
func (g *Graph) GetDependencies(id api.ResourceID) []api.ResourceID {
	return g.AdjacencyList[id]
}

// AllNodes 返回所有节点 ID
func (g *Graph) AllNodes() []api.ResourceID {
	ids := make([]api.ResourceID, 0, len(g.Nodes))
	for id := range g.Nodes {
		ids = append(ids, id)
	}
	return ids
}

// Size 返回节点数量
func (g *Graph) Size() int {
	return len(g.Nodes)
}
