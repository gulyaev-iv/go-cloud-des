package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	dpb "github.com/gulyaev-iv/go-cloud-des/api/dispatcher/v1"
)

type NodeRecord struct {
	Node       *dpb.NodeStatus
	LastSeenAt time.Time
}

type NodeRegistry struct {
	mu  sync.RWMutex
	ttl time.Duration

	nodes map[string]NodeRecord
}

func NewNodeRegistry(ttl time.Duration) *NodeRegistry {
	return &NodeRegistry{
		ttl:   ttl,
		nodes: make(map[string]NodeRecord),
	}
}

func (r *NodeRegistry) Register(node *dpb.NodeStatus) error {
	if node == nil {
		return fmt.Errorf("empty node")
	}
	if node.GetNodeId() == "" {
		return fmt.Errorf("empty node_id")
	}
	if node.GetAddress() == "" {
		return fmt.Errorf("empty node address")
	}
	if node.GetRuntimeGoos() == "" {
		return fmt.Errorf("empty runtime_goos")
	}
	if node.GetRuntimeGoarch() == "" {
		return fmt.Errorf("empty runtime_goarch")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.nodes[node.GetNodeId()] = NodeRecord{
		Node:       cloneNodeStatus(node),
		LastSeenAt: time.Now(),
	}

	return nil
}

func (r *NodeRegistry) Heartbeat(node *dpb.NodeStatus) error {
	if node == nil {
		return fmt.Errorf("empty node")
	}
	if node.GetNodeId() == "" {
		return fmt.Errorf("empty node_id")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.nodes[node.GetNodeId()] = NodeRecord{
		Node:       cloneNodeStatus(node),
		LastSeenAt: time.Now(),
	}

	return nil
}

func (r *NodeRegistry) List(status string) []*dpb.NodeStatus {
	r.mu.Lock()
	defer r.mu.Unlock()

	status = strings.ToUpper(strings.TrimSpace(status))
	now := time.Now()

	result := make([]*dpb.NodeStatus, 0, len(r.nodes))

	for nodeID, record := range r.nodes {
		if now.Sub(record.LastSeenAt) > r.ttl {
			delete(r.nodes, nodeID)
			continue
		}

		if status != "" && status != nodeStatusOnline {
			continue
		}

		result = append(result, cloneNodeStatus(record.Node))
	}

	return result
}

func cloneNodeStatus(node *dpb.NodeStatus) *dpb.NodeStatus {
	if node == nil {
		return nil
	}

	return &dpb.NodeStatus{
		NodeId:              node.GetNodeId(),
		Address:             node.GetAddress(),
		RuntimeGoos:         node.GetRuntimeGoos(),
		RuntimeGoarch:       node.GetRuntimeGoarch(),
		TotalSlots:          node.GetTotalSlots(),
		UsedSlots:           node.GetUsedSlots(),
		TotalMemoryBytes:    node.GetTotalMemoryBytes(),
		ReservedMemoryBytes: node.GetReservedMemoryBytes(),
		ActiveExperiments:   node.GetActiveExperiments(),
		CachedModels:        node.GetCachedModels(),
	}
}

func (r *NodeRegistry) PickAndReserve(goos string, goarch string, memoryLimitBytes uint64) (*dpb.NodeStatus, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	goos = strings.TrimSpace(goos)
	goarch = strings.TrimSpace(goarch)
	now := time.Now()

	for nodeID, record := range r.nodes {
		if now.Sub(record.LastSeenAt) > r.ttl {
			delete(r.nodes, nodeID)
			continue
		}

		node := record.Node
		if node == nil {
			delete(r.nodes, nodeID)
			continue
		}

		if goos != "" && node.GetRuntimeGoos() != goos {
			continue
		}
		if goarch != "" && node.GetRuntimeGoarch() != goarch {
			continue
		}
		if node.GetUsedSlots() >= node.GetTotalSlots() {
			continue
		}
		if !hasEnoughMemory(node, memoryLimitBytes) {
			continue
		}

		selected := cloneNodeStatus(node)

		reserved := cloneNodeStatus(node)
		reserved.UsedSlots++
		reserved.ActiveExperiments++

		if memoryLimitBytes > 0 {
			reserved.ReservedMemoryBytes += memoryLimitBytes
		}

		r.nodes[nodeID] = NodeRecord{
			Node:       reserved,
			LastSeenAt: record.LastSeenAt,
		}

		return selected, true
	}

	return nil, false
}

func (r *NodeRegistry) ReleaseReservation(nodeID string, memoryLimitBytes uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.nodes[nodeID]
	if !ok || record.Node == nil {
		return
	}

	node := cloneNodeStatus(record.Node)

	if node.UsedSlots > 0 {
		node.UsedSlots--
	}
	if node.ActiveExperiments > 0 {
		node.ActiveExperiments--
	}
	if memoryLimitBytes > 0 {
		if node.ReservedMemoryBytes >= memoryLimitBytes {
			node.ReservedMemoryBytes -= memoryLimitBytes
		} else {
			node.ReservedMemoryBytes = 0
		}
	}

	r.nodes[nodeID] = NodeRecord{
		Node:       node,
		LastSeenAt: record.LastSeenAt,
	}
}

func (r *NodeRegistry) Remove(nodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.nodes, nodeID)
}

func hasEnoughMemory(node *dpb.NodeStatus, memoryLimitBytes uint64) bool {
	if memoryLimitBytes == 0 {
		return true
	}

	total := node.GetTotalMemoryBytes()
	if total == 0 {
		return true
	}

	return node.GetReservedMemoryBytes()+memoryLimitBytes <= total
}
