package main

import (
	"sync"
	"time"

	dpb "github.com/gulyaev-iv/go-cloud-des/api/dispatcher/v1"
)

type ScheduleExperimentRecord struct {
	BatchID string

	ModelHash    string
	ExperimentID string

	GOOS   string
	GOARCH string

	BinaryKey    string
	BinarySHA256 string

	SetVars  []*dpb.SetVar
	StopRule *dpb.StopRule

	Metrics      []string
	MetricStep   float64
	StoreMetrics bool

	MemoryLimitBytes uint64
	RunTimeoutMs     uint64

	PendingSince time.Time
}

type PendingQueue struct {
	mu sync.Mutex

	items []*ScheduleExperimentRecord
	index map[string]struct{}
}

func NewPendingQueue() *PendingQueue {
	return &PendingQueue{
		items: make([]*ScheduleExperimentRecord, 0),
		index: make(map[string]struct{}),
	}
}

func (q *PendingQueue) Enqueue(record *ScheduleExperimentRecord) bool {
	if record == nil {
		return false
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	key := queueKey(record.ModelHash, record.ExperimentID)
	if _, ok := q.index[key]; ok {
		return false
	}

	if record.PendingSince.IsZero() {
		record.PendingSince = time.Now()
	}

	q.items = append(q.items, record)
	q.index[key] = struct{}{}

	return true
}

func (q *PendingQueue) EnqueueMany(records []*ScheduleExperimentRecord) int {
	added := 0

	for _, record := range records {
		if q.Enqueue(record) {
			added++
		}
	}

	return added
}

func (q *PendingQueue) PopBatch(limit int) []*ScheduleExperimentRecord {
	q.mu.Lock()
	defer q.mu.Unlock()

	if limit <= 0 || limit > len(q.items) {
		limit = len(q.items)
	}

	result := make([]*ScheduleExperimentRecord, 0, limit)

	for i := 0; i < limit; i++ {
		record := q.items[0]
		q.items = q.items[1:]

		delete(q.index, queueKey(record.ModelHash, record.ExperimentID))

		result = append(result, record)
	}

	return result
}

func (q *PendingQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()

	return len(q.items)
}

func queueKey(modelHash string, experimentID string) string {
	return modelHash + ":" + experimentID
}
