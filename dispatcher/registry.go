package main

import (
	"context"
	"fmt"
	"sync"
)

type ExperimentRecord struct {
	Key ExperimentKey

	Result ExperimentResult

	Cancel context.CancelFunc

	MemoryReserved  uint64
	CancelRequested bool
	CancelReason    string
}

type Registry struct {
	mu sync.Mutex

	totalSlots uint64
	usedSlots  uint64

	totalMemoryBytes    uint64
	reservedMemoryBytes uint64

	experiments map[ExperimentKey]*ExperimentRecord
}

func NewRegistry(totalSlots uint64, totalMemoryBytes uint64) *Registry {
	return &Registry{
		totalSlots:       totalSlots,
		totalMemoryBytes: totalMemoryBytes,
		experiments:      make(map[ExperimentKey]*ExperimentRecord),
	}
}

func (r *Registry) TryStart(modelHash string, experimentID string, memoryLimitBytes uint64) (*ExperimentRecord, string, string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := experimentKey(modelHash, experimentID)

	if existing, ok := r.experiments[key]; ok && isActiveStatus(existing.Result.Status) {
		return nil, "experiment_already_running", fmt.Sprintf("experiment %s/%s is already running", modelHash, experimentID)
	}

	if r.totalSlots > 0 && r.usedSlots >= r.totalSlots {
		return nil, "no_free_slots", "no free execution slots"
	}

	if r.totalMemoryBytes > 0 && memoryLimitBytes > 0 && r.reservedMemoryBytes+memoryLimitBytes > r.totalMemoryBytes {
		return nil, "not_enough_memory", "not enough free memory"
	}

	record := &ExperimentRecord{
		Key: key,
		Result: ExperimentResult{
			ModelHash:        modelHash,
			ExperimentID:     experimentID,
			Status:           statusStarting,
			ExitCode:         -1,
			FinalMetrics:     map[string]string{},
			MemoryLimitBytes: memoryLimitBytes,
			PeakMemoryBytes:  0,
		},
		MemoryReserved: memoryLimitBytes,
	}

	r.experiments[key] = record
	r.usedSlots++
	r.reservedMemoryBytes += memoryLimitBytes

	return record, "", ""
}

func (r *Registry) SetCancel(modelHash string, experimentID string, cancel context.CancelFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := experimentKey(modelHash, experimentID)
	if record, ok := r.experiments[key]; ok {
		record.Cancel = cancel
	}
}

func (r *Registry) SetStatus(modelHash string, experimentID string, status string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := experimentKey(modelHash, experimentID)
	if record, ok := r.experiments[key]; ok {
		record.Result.Status = status
	}
}

func (r *Registry) Cancel(modelHash string, experimentID string) (bool, string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := experimentKey(modelHash, experimentID)
	record, ok := r.experiments[key]
	if !ok {
		return false, "experiment not found"
	}

	if !isActiveStatus(record.Result.Status) {
		return false, "experiment is not active"
	}

	record.CancelRequested = true
	if record.CancelReason == "" {
		record.CancelReason = "user_canceled"
	}
	record.Result.Status = statusCanceled
	record.Result.FinishReason = record.CancelReason

	if record.Cancel != nil {
		record.Cancel()
	}

	return true, "experiment cancellation requested"
}

func (r *Registry) Finish(modelHash string, experimentID string, result ExperimentResult) ExperimentResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := experimentKey(modelHash, experimentID)
	record, ok := r.experiments[key]
	if !ok {
		r.experiments[key] = &ExperimentRecord{
			Key:    key,
			Result: result,
		}
		return result
	}

	if record.CancelRequested && result.Status != statusFinished {
		result.Status = statusCanceled

		if record.CancelReason != "" {
			result.FinishReason = record.CancelReason
		} else if result.FinishReason == "" {
			result.FinishReason = "user_canceled"
		}
	}

	record.Result = result

	if record.MemoryReserved > 0 {
		if r.reservedMemoryBytes >= record.MemoryReserved {
			r.reservedMemoryBytes -= record.MemoryReserved
		} else {
			r.reservedMemoryBytes = 0
		}
		record.MemoryReserved = 0
	}

	if r.usedSlots > 0 {
		r.usedSlots--
	}

	return result
}

func (r *Registry) Get(modelHash string, experimentID string) (ExperimentResult, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	record, ok := r.experiments[experimentKey(modelHash, experimentID)]
	if !ok {
		return ExperimentResult{}, false
	}

	return record.Result, true
}

func (r *Registry) List(status string) []ExperimentResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := make([]ExperimentResult, 0, len(r.experiments))

	for _, record := range r.experiments {
		if status != "" && record.Result.Status != status {
			continue
		}

		result = append(result, record.Result)
	}

	return result
}

func (r *Registry) Snapshot() RegistrySnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()

	var active uint64
	for _, record := range r.experiments {
		if isActiveStatus(record.Result.Status) {
			active++
		}
	}

	return RegistrySnapshot{
		TotalSlots:          r.totalSlots,
		UsedSlots:           r.usedSlots,
		TotalMemoryBytes:    r.totalMemoryBytes,
		ReservedMemoryBytes: r.reservedMemoryBytes,
		ActiveExperiments:   active,
	}
}

type RegistrySnapshot struct {
	TotalSlots          uint64
	UsedSlots           uint64
	TotalMemoryBytes    uint64
	ReservedMemoryBytes uint64
	ActiveExperiments   uint64
}

func (r *Registry) CancelAll(reason string) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0

	for _, record := range r.experiments {
		if !isActiveStatus(record.Result.Status) {
			continue
		}

		record.CancelRequested = true
		if record.CancelReason == "" {
			record.CancelReason = reason
		}
		record.Result.Status = statusCanceled
		if record.Result.FinishReason == "" {
			record.Result.FinishReason = record.CancelReason
		}

		if record.Cancel != nil {
			record.Cancel()
		}

		count++
	}

	return count
}
