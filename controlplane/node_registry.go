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

	ReservedSlots       uint64
	ReservedMemoryBytes uint64

	failureCount uint32
}

type NodeRegistry struct {
	mu  sync.Mutex
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

	var reservedSlots, reservedMemory uint64
	var failureCount uint32

	if existing, ok := r.nodes[node.GetNodeId()]; ok {
		reservedSlots = existing.ReservedSlots
		reservedMemory = existing.ReservedMemoryBytes
		failureCount = existing.failureCount
	}

	r.nodes[node.GetNodeId()] = NodeRecord{
		Node:                cloneNodeStatus(node),
		LastSeenAt:          time.Now(),
		ReservedSlots:       reservedSlots,
		ReservedMemoryBytes: reservedMemory,
		failureCount:        failureCount,
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

	existing, ok := r.nodes[node.GetNodeId()]

	updated := NodeRecord{
		Node:       cloneNodeStatus(node),
		LastSeenAt: time.Now(),
	}

	if ok {
		updated.ReservedSlots = existing.ReservedSlots
		updated.ReservedMemoryBytes = existing.ReservedMemoryBytes
		updated.failureCount = existing.failureCount
	}

	r.nodes[node.GetNodeId()] = updated

	return nil
}

func (r *NodeRegistry) List(status string) (alive []*dpb.NodeStatus, evicted []string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	status = strings.ToUpper(strings.TrimSpace(status))
	now := time.Now()

	alive = make([]*dpb.NodeStatus, 0, len(r.nodes))

	for nodeID, record := range r.nodes {
		if now.Sub(record.LastSeenAt) > r.ttl {
			delete(r.nodes, nodeID)
			evicted = append(evicted, nodeID)
			continue
		}

		if status != "" && status != nodeStatusOnline {
			continue
		}

		alive = append(alive, cloneNodeStatus(record.Node))
	}

	return alive, evicted
}

func (r *NodeRegistry) ActiveNodeIDs() map[string]struct{} {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	result := make(map[string]struct{}, len(r.nodes))

	for nodeID, record := range r.nodes {
		if now.Sub(record.LastSeenAt) > r.ttl {
			continue
		}
		result[nodeID] = struct{}{}
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

		effectiveUsedSlots := node.GetUsedSlots()
		if record.ReservedSlots > effectiveUsedSlots {
			effectiveUsedSlots = record.ReservedSlots
		}
		if effectiveUsedSlots >= node.GetTotalSlots() {
			continue
		}

		effectiveReservedMemory := node.GetReservedMemoryBytes()
		if record.ReservedMemoryBytes > effectiveReservedMemory {
			effectiveReservedMemory = record.ReservedMemoryBytes
		}
		if !hasEnoughMemoryAt(node, effectiveReservedMemory, memoryLimitBytes) {
			continue
		}

		selected := cloneNodeStatus(node)

		record.ReservedSlots++
		if memoryLimitBytes > 0 {
			record.ReservedMemoryBytes += memoryLimitBytes
		}

		r.nodes[nodeID] = record

		return selected, true
	}

	return nil, false
}

func (r *NodeRegistry) ReleaseReservation(nodeID string, memoryLimitBytes uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.nodes[nodeID]
	if !ok {
		return
	}

	if record.ReservedSlots > 0 {
		record.ReservedSlots--
	}
	if memoryLimitBytes > 0 {
		if record.ReservedMemoryBytes >= memoryLimitBytes {
			record.ReservedMemoryBytes -= memoryLimitBytes
		} else {
			record.ReservedMemoryBytes = 0
		}
	}

	r.nodes[nodeID] = record
}

func (r *NodeRegistry) Remove(nodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.nodes, nodeID)
}

func (r *NodeRegistry) MarkNodeFailed(nodeID string, maxFailures uint32) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.nodes[nodeID]
	if !ok {
		return false
	}

	record.failureCount++
	r.nodes[nodeID] = record

	if record.failureCount >= maxFailures {
		delete(r.nodes, nodeID)
		return true
	}

	return false
}

func (r *NodeRegistry) MarkNodeSucceeded(nodeID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.nodes[nodeID]
	if !ok {
		return
	}
	if record.failureCount == 0 {
		return
	}
	record.failureCount = 0
	r.nodes[nodeID] = record
}

func hasEnoughMemoryAt(node *dpb.NodeStatus, currentReserved uint64, memoryLimitBytes uint64) bool {
	if memoryLimitBytes == 0 {
		return true
	}

	total := node.GetTotalMemoryBytes()
	if total == 0 {
		return true
	}

	return currentReserved+memoryLimitBytes <= total
}

func hasEnoughMemory(node *dpb.NodeStatus, memoryLimitBytes uint64) bool {
	return hasEnoughMemoryAt(node, node.GetReservedMemoryBytes(), memoryLimitBytes)
}

func (r *NodeRegistry) Get(nodeID string) (*dpb.NodeStatus, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.nodes[nodeID]
	if !ok {
		return nil, false
	}

	if time.Since(record.LastSeenAt) > r.ttl {
		delete(r.nodes, nodeID)
		return nil, false
	}

	if record.Node == nil {
		delete(r.nodes, nodeID)
		return nil, false
	}

	return cloneNodeStatus(record.Node), true
}
