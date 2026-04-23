package dag

import (
	"fmt"

	"orchestrator/pkg/api"
)

// TopologicalSort 拓扑排序
// 返回按依赖顺序排列的节点 ID 列表
func (g *Graph) TopologicalSort() ([]api.ResourceID, error) {
	// 计算入度
	inDegree := make(map[api.ResourceID]int)
	for id := range g.Nodes {
		inDegree[id] = 0
	}

	// 入度计算在后面重新进行

	// 重新计算入度：有多少依赖指向当前节点
	for id, node := range g.Nodes {
		inDegree[id] = len(node.Dependencies)
	}

	// 使用 Kahn 算法进行拓扑排序
	var result []api.ResourceID
	queue := make([]api.ResourceID, 0)

	// 找到所有入度为 0 的节点
	for id, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	for len(queue) > 0 {
		// 取出队首节点
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// 减少所有依赖当前节点的节点的入度
		for _, dependent := range g.ReverseAdj[current] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// 检查是否存在环
	if len(result) != len(g.Nodes) {
		return nil, fmt.Errorf("cycle detected in DAG")
	}

	return result, nil
}

// DetectCycle 检测图中是否存在环
func (g *Graph) DetectCycle() bool {
	visited := make(map[api.ResourceID]bool)
	recStack := make(map[api.ResourceID]bool)

	for id := range g.Nodes {
		if !visited[id] {
			if g.detectCycleDFS(id, visited, recStack) {
				return true
			}
		}
	}
	return false
}

func (g *Graph) detectCycleDFS(id api.ResourceID, visited, recStack map[api.ResourceID]bool) bool {
	visited[id] = true
	recStack[id] = true

	for _, dep := range g.AdjacencyList[id] {
		if !visited[dep] {
			if g.detectCycleDFS(dep, visited, recStack) {
				return true
			}
		} else if recStack[dep] {
			return true
		}
	}

	recStack[id] = false
	return false
}

// GetExecutionLayers 获取执行层级
// 每一层内的节点可以并行执行
// 返回二维切片，每一维代表一个执行层级
func (g *Graph) GetExecutionLayers() ([][]api.ResourceID, error) {
	// 计算每个节点的层级
	layer := make(map[api.ResourceID]int)
	for id := range g.Nodes {
		layer[id] = 0
	}

	// 拓扑排序
	sorted, err := g.TopologicalSort()
	if err != nil {
		return nil, err
	}

	// 计算层级：节点的层级 = max(依赖节点层级) + 1
	maxLayer := 0
	for _, id := range sorted {
		node := g.Nodes[id]
		nodeLayer := 0
		for _, dep := range node.Dependencies {
			if layer[dep]+1 > nodeLayer {
				nodeLayer = layer[dep] + 1
			}
		}
		layer[id] = nodeLayer
		if nodeLayer > maxLayer {
			maxLayer = nodeLayer
		}
	}

	// 按层级分组
	layers := make([][]api.ResourceID, maxLayer+1)
	for id, l := range layer {
		layers[l] = append(layers[l], id)
	}

	return layers, nil
}

// Validate 验证 DAG 的合法性
func (g *Graph) Validate() error {
	// 检查是否存在环
	if g.DetectCycle() {
		return fmt.Errorf("DAG contains cycle")
	}

	// 检查所有依赖是否存在
	for id, node := range g.Nodes {
		for _, dep := range node.Dependencies {
			if _, exists := g.Nodes[dep]; !exists {
				return fmt.Errorf("node %s depends on non-existent node %s", id, dep)
			}
		}
	}

	return nil
}
