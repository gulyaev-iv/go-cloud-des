package main

import (
	"context"
	"log"
	"time"

	dpb "github.com/gulyaev-iv/go-cloud-des/api/dispatcher/v1"
)

type Scheduler struct {
	cfg Config

	repo  *Repository
	nodes *NodeRegistry
	queue *PendingQueue

	provisioner NodeProvisioner

	lastScaleOut time.Time
}

func NewScheduler(cfg Config, repo *Repository, nodes *NodeRegistry, provisioner NodeProvisioner) *Scheduler {
	return &Scheduler{
		cfg:         cfg,
		repo:        repo,
		nodes:       nodes,
		queue:       NewPendingQueue(),
		provisioner: provisioner,
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.SchedulerInterval)
	defer ticker.Stop()

	log.Printf(
		"scheduler started: interval=%s scale_out_after=%s scale_out_cooldown=%s",
		s.cfg.SchedulerInterval,
		s.cfg.ScaleOutAfter,
		s.cfg.ScaleOutCooldown,
	)

	for {
		select {
		case <-ctx.Done():
			log.Printf("scheduler stopped")
			return

		case <-ticker.C:
			s.RunOnce(ctx)
		}
	}
}

func (s *Scheduler) RunOnce(ctx context.Context) {
	records := s.queue.PopBatch(s.queue.Len())
	if len(records) == 0 {
		return
	}

	scheduled := 0
	noNode := 0

	var firstBlocked *ScheduleExperimentRecord

	for _, record := range records {
		node, ok := s.nodes.PickAndReserve(record.GOOS, record.GOARCH, record.MemoryLimitBytes)
		if !ok {
			noNode++
			if firstBlocked == nil {
				firstBlocked = record
			}
			s.queue.Enqueue(record)
			continue
		}

		acquired, err := s.repo.TryMarkExperimentStarting(ctx, record.ModelHash, record.ExperimentID, node.GetNodeId())
		if err != nil {
			s.nodes.ReleaseReservation(node.GetNodeId(), record.MemoryLimitBytes)
			s.queue.Enqueue(record)

			log.Printf(
				"scheduler mark starting failed: model_hash=%s experiment_id=%s error=%v",
				record.ModelHash,
				record.ExperimentID,
				err,
			)
			continue
		}

		if !acquired {
			s.nodes.ReleaseReservation(node.GetNodeId(), record.MemoryLimitBytes)
			continue
		}

		accepted, err := s.startExperimentOnNode(ctx, record, node)
		if err != nil {
			s.nodes.ReleaseReservation(node.GetNodeId(), record.MemoryLimitBytes)
			s.nodes.Remove(node.GetNodeId())

			if markErr := s.repo.MarkExperimentPending(ctx, record.ModelHash, record.ExperimentID, err.Error()); markErr != nil {
				log.Printf(
					"scheduler rollback experiment to pending failed: model_hash=%s experiment_id=%s error=%v",
					record.ModelHash,
					record.ExperimentID,
					markErr,
				)
				continue
			}

			s.queue.Enqueue(record)

			log.Printf(
				"dispatcher start failed: node_id=%s address=%s model_hash=%s experiment_id=%s error=%v",
				node.GetNodeId(),
				node.GetAddress(),
				record.ModelHash,
				record.ExperimentID,
				err,
			)
			continue
		}

		if !accepted {
			s.nodes.ReleaseReservation(node.GetNodeId(), record.MemoryLimitBytes)
			continue
		}

		if err := s.repo.RefreshBatchStatus(ctx, record.BatchID); err != nil {
			log.Printf("refresh batch status failed: batch_id=%s error=%v", record.BatchID, err)
		}

		scheduled++
	}

	if noNode > 0 {
		s.maybeRequestScaleOut(ctx, firstBlocked)
	}

	if scheduled > 0 || noNode > 0 {
		log.Printf(
			"scheduler tick completed: loaded=%d scheduled=%d no_node=%d queue_len=%d",
			len(records),
			scheduled,
			noNode,
			s.queue.Len(),
		)
	}
}

func (s *Scheduler) Enqueue(records []*ScheduleExperimentRecord) int {
	added := s.queue.EnqueueMany(records)

	if added > 0 {
		log.Printf("scheduler enqueued experiments: added=%d queue_len=%d", added, s.queue.Len())
	}

	return added
}

func (s *Scheduler) startExperimentOnNode(ctx context.Context, record *ScheduleExperimentRecord, node *dpb.NodeStatus) (bool, error) {
	client, err := NewDispatcherClient(node.GetAddress(), s.cfg.DispatcherRequestTimeout)
	if err != nil {
		return false, err
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("dispatcher client close error: node_id=%s address=%s error=%v", node.GetNodeId(), node.GetAddress(), err)
		}
	}()

	req := &dpb.StartExperimentRequest{
		ModelHash:    record.ModelHash,
		ExperimentId: record.ExperimentID,
		Binary: &dpb.BinaryArtifact{
			Key:    record.BinaryKey,
			Sha256: record.BinarySHA256,
		},
		SetVars:          record.SetVars,
		StopRule:         record.StopRule,
		Metrics:          record.Metrics,
		MetricStep:       record.MetricStep,
		StoreMetrics:     record.StoreMetrics,
		MemoryLimitBytes: record.MemoryLimitBytes,
		RunTimeoutMs:     record.RunTimeoutMs,
	}

	resp, err := client.StartExperiment(ctx, req)
	if err != nil {
		return false, err
	}

	if !resp.GetAccepted() {
		message := resp.GetMessage()
		if message == "" {
			message = resp.GetErrorCode()
		}
		if message == "" {
			message = "dispatcher rejected experiment"
		}

		if err := s.repo.MarkExperimentFailedToStart(ctx, record.ModelHash, record.ExperimentID, node.GetNodeId(), message); err != nil {
			return false, err
		}

		if err := s.repo.RefreshBatchStatus(ctx, record.BatchID); err != nil {
			return false, err
		}

		log.Printf(
			"experiment rejected by dispatcher: node_id=%s model_hash=%s experiment_id=%s error_code=%s message=%s",
			node.GetNodeId(),
			record.ModelHash,
			record.ExperimentID,
			resp.GetErrorCode(),
			resp.GetMessage(),
		)

		return false, nil
	}

	log.Printf(
		"experiment started: node_id=%s address=%s batch_id=%s model_hash=%s experiment_id=%s status=%s",
		node.GetNodeId(),
		node.GetAddress(),
		record.BatchID,
		record.ModelHash,
		record.ExperimentID,
		resp.GetStatus(),
	)

	return true, nil
}

func (s *Scheduler) maybeRequestScaleOut(ctx context.Context, record *ScheduleExperimentRecord) {
	if s.provisioner == nil || record == nil {
		return
	}

	oldestPendingFor := time.Since(record.PendingSince)
	if oldestPendingFor < s.cfg.ScaleOutAfter {
		return
	}

	if !s.lastScaleOut.IsZero() && time.Since(s.lastScaleOut) < s.cfg.ScaleOutCooldown {
		return
	}

	s.lastScaleOut = time.Now()

	if err := s.provisioner.RequestScaleOut(ctx, ScaleOutRequest{
		PendingExperiments: s.queue.Len(),
		NeedNodes:          1,
		GOOS:               record.GOOS,
		GOARCH:             record.GOARCH,
		MemoryLimitBytes:   record.MemoryLimitBytes,
		OldestPendingFor:   oldestPendingFor,
	}); err != nil {
		log.Printf("scale out request failed: %v", err)
	}
}
