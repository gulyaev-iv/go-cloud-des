package main

import (
	"context"
	"log"

	cpb "github.com/gulyaev-iv/go-cloud-des/api/controlplane/v1"
	dpb "github.com/gulyaev-iv/go-cloud-des/api/dispatcher/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ControlPlaneServer struct {
	cpb.UnimplementedControlPlaneServiceServer
	cpb.UnimplementedControlPlaneNodeServiceServer

	cfg Config

	repo      *Repository
	nodes     *NodeRegistry
	codegen   *CodegenClient
	scheduler *Scheduler
}

func NewControlPlaneServer(cfg Config, repo *Repository, nodes *NodeRegistry, codegen *CodegenClient, scheduler *Scheduler) *ControlPlaneServer {
	return &ControlPlaneServer{
		cfg:       cfg,
		repo:      repo,
		nodes:     nodes,
		codegen:   codegen,
		scheduler: scheduler,
	}
}
func (s *ControlPlaneServer) GetExperimentBatch(ctx context.Context, req *cpb.GetExperimentBatchRequest) (*cpb.ExperimentBatchStatus, error) {
	if req.GetBatchId() == "" {
		return nil, status.Error(codes.InvalidArgument, "empty batch_id")
	}

	batch, ok, err := s.repo.GetExperimentBatch(ctx, req.GetBatchId())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, status.Error(codes.NotFound, "experiment batch not found")
	}

	return batch, nil
}

func (s *ControlPlaneServer) GetExperiment(ctx context.Context, req *cpb.GetExperimentRequest) (*dpb.ExperimentStatus, error) {
	if req.GetModelHash() == "" {
		return nil, status.Error(codes.InvalidArgument, "empty model_hash")
	}
	if req.GetExperimentId() == "" {
		return nil, status.Error(codes.InvalidArgument, "empty experiment_id")
	}

	experiment, ok, err := s.repo.GetExperiment(ctx, req.GetModelHash(), req.GetExperimentId())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, status.Error(codes.NotFound, "experiment not found")
	}

	return experiment, nil
}

func (s *ControlPlaneServer) ListNodes(ctx context.Context, req *cpb.ListNodesRequest) (*cpb.ListNodesResponse, error) {
	nodes, evicted := s.nodes.List(req.GetStatus())

	for _, nodeID := range evicted {
		log.Printf("dispatcher node evicted by ttl: node_id=%s", nodeID)
	}

	return &cpb.ListNodesResponse{
		Nodes: nodes,
	}, nil
}

func (s *ControlPlaneServer) RegisterNode(ctx context.Context, req *cpb.RegisterNodeRequest) (*cpb.RegisterNodeResponse, error) {
	node := req.GetNode()

	if err := s.nodes.Register(node); err != nil {
		log.Printf("dispatcher node registration rejected: error=%v", err)

		return &cpb.RegisterNodeResponse{
			Accepted: false,
			Message:  err.Error(),
		}, nil
	}

	log.Printf(
		"dispatcher node registered: node_id=%s address=%s goos=%s goarch=%s slots=%d memory=%d",
		node.GetNodeId(),
		node.GetAddress(),
		node.GetRuntimeGoos(),
		node.GetRuntimeGoarch(),
		node.GetTotalSlots(),
		node.GetTotalMemoryBytes(),
	)

	return &cpb.RegisterNodeResponse{
		Accepted: true,
		Message:  "node registered",
	}, nil
}

func (s *ControlPlaneServer) SendHeartbeat(ctx context.Context, req *cpb.NodeHeartbeat) (*cpb.HeartbeatResponse, error) {
	node := req.GetNode()

	if err := s.nodes.Heartbeat(node); err != nil {
		log.Printf("dispatcher heartbeat rejected: error=%v", err)

		return &cpb.HeartbeatResponse{
			Ok:      false,
			Message: err.Error(),
		}, nil
	}

	return &cpb.HeartbeatResponse{
		Ok:      true,
		Message: "heartbeat accepted",
	}, nil
}

func (s *ControlPlaneServer) ReportExperimentResult(ctx context.Context, req *cpb.ReportExperimentResultRequest) (*cpb.ReportExperimentResultResponse, error) {
	result := req.GetResult()
	if result == nil {
		return &cpb.ReportExperimentResultResponse{
			Ok:      false,
			Message: "empty experiment result",
		}, nil
	}

	batchID, found, err := s.repo.UpdateExperimentResult(ctx, req.GetNodeId(), result)
	if err != nil {
		log.Printf(
			"experiment result update failed: node_id=%s model_hash=%s experiment_id=%s error=%v",
			req.GetNodeId(),
			result.GetModelHash(),
			result.GetExperimentId(),
			err,
		)

		return &cpb.ReportExperimentResultResponse{
			Ok:      false,
			Message: err.Error(),
		}, nil
	}

	if !found {
		log.Printf(
			"experiment result rejected: node_id=%s model_hash=%s experiment_id=%s reason=experiment_not_found",
			req.GetNodeId(),
			result.GetModelHash(),
			result.GetExperimentId(),
		)

		return &cpb.ReportExperimentResultResponse{
			Ok:      false,
			Message: "experiment not found",
		}, nil
	}

	s.nodes.ReleaseReservation(req.GetNodeId(), result.GetMemoryLimitBytes())

	log.Printf(
		"experiment result accepted: node_id=%s batch_id=%s model_hash=%s experiment_id=%s status=%s",
		req.GetNodeId(),
		batchID,
		result.GetModelHash(),
		result.GetExperimentId(),
		result.GetStatus(),
	)

	return &cpb.ReportExperimentResultResponse{
		Ok:      true,
		Message: "experiment result accepted",
	}, nil
}

func (s *ControlPlaneServer) CancelExperiment(ctx context.Context, req *cpb.CancelExperimentRequest) (*cpb.CancelExperimentResponse, error) {
	if req.GetModelHash() == "" {
		return nil, status.Error(codes.InvalidArgument, "empty model_hash")
	}
	if req.GetExperimentId() == "" {
		return nil, status.Error(codes.InvalidArgument, "empty experiment_id")
	}

	record, found, err := s.repo.GetExperimentForCancel(ctx, req.GetModelHash(), req.GetExperimentId())
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, status.Error(codes.NotFound, "experiment not found")
	}

	switch record.Status {
	case statusPending:
		if s.scheduler != nil {
			s.scheduler.Remove(record.ModelHash, record.ExperimentID)
		}

		canceled, err := s.repo.MarkPendingExperimentCanceled(ctx, record.ModelHash, record.ExperimentID)
		if err != nil {
			return nil, err
		}

		if canceled {
			if err := s.repo.RefreshBatchStatus(ctx, record.BatchID); err != nil {
				return nil, err
			}

			log.Printf(
				"pending experiment canceled: batch_id=%s model_hash=%s experiment_id=%s",
				record.BatchID,
				record.ModelHash,
				record.ExperimentID,
			)

			return &cpb.CancelExperimentResponse{
				Canceled:     true,
				ModelHash:    record.ModelHash,
				ExperimentId: record.ExperimentID,
				Status:       statusCanceled,
				Message:      "pending experiment canceled",
			}, nil
		}

		return &cpb.CancelExperimentResponse{
			Canceled:     false,
			ModelHash:    record.ModelHash,
			ExperimentId: record.ExperimentID,
			Status:       record.Status,
			Message:      "experiment is no longer pending",
		}, nil

	case statusStarting, statusRunning:
		if record.DispatcherNodeID == "" {
			return &cpb.CancelExperimentResponse{
				Canceled:     false,
				ModelHash:    record.ModelHash,
				ExperimentId: record.ExperimentID,
				Status:       record.Status,
				Message:      "experiment has no dispatcher node",
			}, nil
		}

		node, ok := s.nodes.Get(record.DispatcherNodeID)
		if !ok {
			return &cpb.CancelExperimentResponse{
				Canceled:     false,
				ModelHash:    record.ModelHash,
				ExperimentId: record.ExperimentID,
				Status:       record.Status,
				Message:      "dispatcher node is not available",
			}, nil
		}

		client, err := NewDispatcherClient(node.GetAddress(), s.cfg.DispatcherRequestTimeout)
		if err != nil {
			return nil, err
		}
		defer func() {
			if err := client.Close(); err != nil {
				log.Printf("dispatcher client close error: node_id=%s address=%s error=%v", node.GetNodeId(), node.GetAddress(), err)
			}
		}()

		resp, err := client.StopExperiment(ctx, &dpb.StopExperimentRequest{
			ModelHash:    record.ModelHash,
			ExperimentId: record.ExperimentID,
		})
		if err != nil {
			return nil, err
		}

		log.Printf(
			"active experiment cancel requested: node_id=%s model_hash=%s experiment_id=%s stopped=%t dispatcher_message=%s",
			node.GetNodeId(),
			record.ModelHash,
			record.ExperimentID,
			resp.GetStopped(),
			resp.GetMessage(),
		)

		message := "cancellation requested; terminal status will be set when dispatcher reports result"
		if !resp.GetStopped() {
			message = resp.GetMessage()
			if message == "" {
				message = "dispatcher could not stop experiment"
			}
		}

		return &cpb.CancelExperimentResponse{
			Canceled:     false,
			ModelHash:    record.ModelHash,
			ExperimentId: record.ExperimentID,
			Status:       record.Status,
			Message:      message,
		}, nil

	case statusCanceled:
		return &cpb.CancelExperimentResponse{
			Canceled:     true,
			ModelHash:    record.ModelHash,
			ExperimentId: record.ExperimentID,
			Status:       statusCanceled,
			Message:      "experiment is already canceled",
		}, nil

	case statusFinished, statusFailed:
		return &cpb.CancelExperimentResponse{
			Canceled:     false,
			ModelHash:    record.ModelHash,
			ExperimentId: record.ExperimentID,
			Status:       record.Status,
			Message:      "experiment is already terminal",
		}, nil

	default:
		return &cpb.CancelExperimentResponse{
			Canceled:     false,
			ModelHash:    record.ModelHash,
			ExperimentId: record.ExperimentID,
			Status:       record.Status,
			Message:      "experiment cannot be canceled in current status",
		}, nil
	}
}
