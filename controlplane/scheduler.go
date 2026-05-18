package main

import (
	"context"
	"log"
	"time"

	dpb "github.com/gulyaev-iv/go-cloud-des/api/dispatcher/v1"
)

type startOutcome int

const (
	startOutcomeAccepted startOutcome = iota
	startOutcomeRetryable
	startOutcomeFatal
	startOutcomeAlreadyRunning
	startOutcomeAmbiguous
)

func isTransientRejectCode(code string) bool {
	switch code {
	case "no_free_slots",
		"not_enough_memory",
		"request_canceled":
		return true
	default:
		return false
	}
}

func isAlreadyRunningRejectCode(code string) bool {
	return code == "experiment_already_running"
}

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
	scheduleTicker := time.NewTicker(s.cfg.SchedulerInterval)
	defer scheduleTicker.Stop()

	reconcileTicker := time.NewTicker(s.cfg.NodeLostReconcileInterval)
	defer reconcileTicker.Stop()

	log.Printf(
		"scheduler started: interval=%s scale_out_after=%s scale_out_cooldown=%s max_node_failures=%d reconcile_interval=%s grace_period=%s",
		s.cfg.SchedulerInterval,
		s.cfg.ScaleOutAfter,
		s.cfg.ScaleOutCooldown,
		s.cfg.MaxNodeFailures,
		s.cfg.NodeLostReconcileInterval,
		s.cfg.NodeLostGracePeriod,
	)

	for {
		select {
		case <-ctx.Done():
			log.Printf("scheduler stopped")
			return

		case <-scheduleTicker.C:
			s.RunOnce(ctx)

		case <-reconcileTicker.C:
			s.ReconcileLostNodes(ctx)
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

			log.Printf(
				"scheduler skipped experiment (not in PENDING anymore): model_hash=%s experiment_id=%s",
				record.ModelHash,
				record.ExperimentID,
			)
			continue
		}

		outcome, message, startErr := s.startExperimentOnNode(ctx, record, node)

		switch outcome {
		case startOutcomeAccepted:
			s.nodes.MarkNodeSucceeded(node.GetNodeId())

			if err := s.repo.RefreshBatchStatus(ctx, record.BatchID); err != nil {
				log.Printf("refresh batch status failed: batch_id=%s error=%v", record.BatchID, err)
			}

			scheduled++

		case startOutcomeRetryable:
			s.nodes.ReleaseReservation(node.GetNodeId(), record.MemoryLimitBytes)
			s.nodes.MarkNodeSucceeded(node.GetNodeId())

			if markErr := s.repo.MarkExperimentPending(ctx, record.ModelHash, record.ExperimentID, message); markErr != nil {
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
				"dispatcher transient reject, requeued: node_id=%s model_hash=%s experiment_id=%s message=%s",
				node.GetNodeId(),
				record.ModelHash,
				record.ExperimentID,
				message,
			)

		case startOutcomeFatal:
			s.nodes.ReleaseReservation(node.GetNodeId(), record.MemoryLimitBytes)
			s.nodes.MarkNodeSucceeded(node.GetNodeId())

			if markErr := s.repo.MarkExperimentFailedToStart(ctx, record.ModelHash, record.ExperimentID, node.GetNodeId(), message); markErr != nil {
				log.Printf(
					"scheduler mark experiment failed to start: model_hash=%s experiment_id=%s error=%v",
					record.ModelHash,
					record.ExperimentID,
					markErr,
				)
				continue
			}

			if refreshErr := s.repo.RefreshBatchStatus(ctx, record.BatchID); refreshErr != nil {
				log.Printf("refresh batch status failed: batch_id=%s error=%v", record.BatchID, refreshErr)
			}

			log.Printf(
				"experiment rejected permanently by dispatcher: node_id=%s model_hash=%s experiment_id=%s message=%s",
				node.GetNodeId(),
				record.ModelHash,
				record.ExperimentID,
				message,
			)
		case startOutcomeAlreadyRunning:
			s.nodes.MarkNodeSucceeded(node.GetNodeId())

			if err := s.repo.RefreshBatchStatus(ctx, record.BatchID); err != nil {
				log.Printf("refresh batch status failed: batch_id=%s error=%v", record.BatchID, err)
			}

			scheduled++

			log.Printf(
				"experiment already running on dispatcher, left STARTING: node_id=%s address=%s batch_id=%s model_hash=%s experiment_id=%s message=%s",
				node.GetNodeId(),
				node.GetAddress(),
				record.BatchID,
				record.ModelHash,
				record.ExperimentID,
				message,
			)

		case startOutcomeAmbiguous:
			removed := s.nodes.MarkNodeFailed(node.GetNodeId(), uint32(s.cfg.MaxNodeFailures))
			if removed {
				log.Printf(
					"dispatcher node removed after consecutive failures: node_id=%s address=%s max_failures=%d",
					node.GetNodeId(),
					node.GetAddress(),
					s.cfg.MaxNodeFailures,
				)
			}

			if err := s.repo.RefreshBatchStatus(ctx, record.BatchID); err != nil {
				log.Printf("refresh batch status failed: batch_id=%s error=%v", record.BatchID, err)
			}

			log.Printf(
				"dispatcher start outcome ambiguous, left experiment STARTING: node_id=%s address=%s batch_id=%s model_hash=%s experiment_id=%s error=%v",
				node.GetNodeId(),
				node.GetAddress(),
				record.BatchID,
				record.ModelHash,
				record.ExperimentID,
				startErr,
			)
		}
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

func (s *Scheduler) startExperimentOnNode(ctx context.Context, record *ScheduleExperimentRecord, node *dpb.NodeStatus) (startOutcome, string, error) {
	client, err := NewDispatcherClient(node.GetAddress(), s.cfg.DispatcherRequestTimeout)
	if err != nil {
		return startOutcomeAmbiguous, err.Error(), err
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
		return startOutcomeAmbiguous, err.Error(), err
	}

	if !resp.GetAccepted() {
		message := resp.GetMessage()
		if message == "" {
			message = resp.GetErrorCode()
		}
		if message == "" {
			message = "dispatcher rejected experiment"
		}

		if isAlreadyRunningRejectCode(resp.GetErrorCode()) {
			return startOutcomeAlreadyRunning, message, nil
		}

		if isTransientRejectCode(resp.GetErrorCode()) {
			return startOutcomeRetryable, message, nil
		}

		return startOutcomeFatal, message, nil
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

	return startOutcomeAccepted, "", nil
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

func (s *Scheduler) Remove(modelHash string, experimentID string) bool {
	if s == nil || s.queue == nil {
		return false
	}

	removed := s.queue.Remove(modelHash, experimentID)
	if removed {
		log.Printf(
			"scheduler removed pending experiment: model_hash=%s experiment_id=%s queue_len=%d",
			modelHash,
			experimentID,
			s.queue.Len(),
		)
	}

	return removed
}

func (s *Scheduler) ReconcileLostNodes(ctx context.Context) {
	activeMap := s.nodes.ActiveNodeIDs()

	activeIDs := make([]string, 0, len(activeMap))
	for nodeID := range activeMap {
		activeIDs = append(activeIDs, nodeID)
	}

	lost, err := s.repo.MarkOrphanedExperimentsFailed(ctx, activeIDs, s.cfg.NodeLostGracePeriod)
	if err != nil {
		log.Printf("reconcile lost nodes failed: error=%v", err)
		return
	}

	if len(lost) == 0 {
		return
	}

	batches := make(map[string]struct{}, len(lost))

	for _, item := range lost {
		batches[item.BatchID] = struct{}{}

		log.Printf(
			"experiment marked FAILED (node_lost): batch_id=%s model_hash=%s experiment_id=%s lost_node_id=%s",
			item.BatchID,
			item.ModelHash,
			item.ExperimentID,
			item.DispatcherNodeID,
		)
	}

	for batchID := range batches {
		if err := s.repo.RefreshBatchStatus(ctx, batchID); err != nil {
			log.Printf("refresh batch status after reconcile failed: batch_id=%s error=%v", batchID, err)
		}
	}

	log.Printf("reconciled lost experiments: count=%d batches=%d active_nodes=%d", len(lost), len(batches), len(activeIDs))
}
