package model

import "fmt"

type NodeType string

const (
	NodeTypeTask      NodeType = "task"
	NodeTypeApproval  NodeType = "approval"
	NodeTypeCondition NodeType = "condition"
	NodeTypeMap       NodeType = "map"
)

// DAG 工作流有向无环图定义
type DAG struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// Node 节点定义
type Node struct {
	ID     string                 `json:"id"`
	Type   NodeType               `json:"type"`
	Config map[string]interface{} `json:"config"` // 节点特定配置，例如 service, method
}

// EdgeType 边类型：空/success 成功走此边, error 失败走此边, always 无论成功失败都走
type EdgeType string

const (
	EdgeTypeSuccess EdgeType = "" // 默认
	EdgeTypeError   EdgeType = "error"
	EdgeTypeAlways  EdgeType = "always"
)

// Edge 边定义
type Edge struct {
	From       string   `json:"from"`
	To         string   `json:"to"`
	Condition  string   `json:"condition"`  // 表达式，为空表示无条件执行
	Type       EdgeType `json:"type"`       // ""(默认成功走此边), "error"(失败走此边), "always"(无论成功失败都走)
	IsLoopback bool     `json:"is_loopback"` // 明确标记为合法回退边，放行环路检测
}

// GetNode 获取节点
func (d *DAG) GetNode(id string) *Node {
	for _, n := range d.Nodes {
		if n.ID == id {
			return &n
		}
	}
	return nil
}

// GetStartNodes 获取所有没有入边的起始节点
func (d *DAG) GetStartNodes() []Node {
	hasIncoming := make(map[string]bool)
	for _, e := range d.Edges {
		hasIncoming[e.To] = true
	}
	var starts []Node
	for _, n := range d.Nodes {
		if !hasIncoming[n.ID] {
			starts = append(starts, n)
		}
	}
	return starts
}

// GetOutgoingEdges 获取某个节点的所有出边
func (d *DAG) GetOutgoingEdges(nodeID string) []Edge {
	var edges []Edge
	for _, e := range d.Edges {
		if e.From == nodeID {
			edges = append(edges, e)
		}
	}
	return edges
}

// Validate 验证 DAG 是否合法：无环、边引用有效节点、存在起始节点
func (d *DAG) Validate() error {
	if len(d.Nodes) == 0 {
		return fmt.Errorf("DAG 至少需要一个节点")
	}
	nodeSet := make(map[string]bool)
	for _, n := range d.Nodes {
		nodeSet[n.ID] = true
	}
	for _, e := range d.Edges {
		if !nodeSet[e.From] {
			return fmt.Errorf("边引用不存在的源节点: %s", e.From)
		}
		if !nodeSet[e.To] {
			return fmt.Errorf("边引用不存在的目标节点: %s", e.To)
		}
	}
	if len(d.GetStartNodes()) == 0 {
		return fmt.Errorf("DAG 必须至少有一个起始节点（无入边的节点）")
	}
	if err := d.detectCycle(); err != nil {
		return err
	}
	return nil
}

// detectCycle 使用 DFS 检测有向图中是否存在环，IsLoopback 的边不参与环路检测
func (d *DAG) detectCycle() error {
	adj := make(map[string][]string)
	for _, e := range d.Edges {
		if e.IsLoopback {
			continue
		}
		adj[e.From] = append(adj[e.From], e.To)
	}
	state := make(map[string]int) // 0=未访问 1=访问中 2=已完成
	for _, n := range d.Nodes {
		state[n.ID] = 0
	}
	var visit func(nodeID string) error
	visit = func(nodeID string) error {
		if state[nodeID] == 1 {
			return fmt.Errorf("DAG 存在环，涉及节点: %s", nodeID)
		}
		if state[nodeID] == 2 {
			return nil
		}
		state[nodeID] = 1
		for _, next := range adj[nodeID] {
			if err := visit(next); err != nil {
				return err
			}
		}
		state[nodeID] = 2
		return nil
	}
	for _, n := range d.Nodes {
		if state[n.ID] == 0 {
			if err := visit(n.ID); err != nil {
				return err
			}
		}
	}
	return nil
}
