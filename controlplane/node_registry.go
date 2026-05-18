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
	r.mu.RLock()
	defer r.mu.RUnlock()

	status = strings.ToUpper(strings.TrimSpace(status))
	now := time.Now()

	result := make([]*dpb.NodeStatus, 0, len(r.nodes))

	for _, record := range r.nodes {
		currentStatus := r.status(record, now)
		if status != "" && status != currentStatus {
			continue
		}

		result = append(result, cloneNodeStatus(record.Node))
	}

	return result
}

func (r *NodeRegistry) status(record NodeRecord, now time.Time) string {
	if now.Sub(record.LastSeenAt) <= r.ttl {
		return nodeStatusOnline
	}

	return nodeStatusStale
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
