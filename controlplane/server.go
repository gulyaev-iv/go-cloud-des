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

	repo  *Repository
	nodes *NodeRegistry
}

func NewControlPlaneServer(cfg Config, repo *Repository, nodes *NodeRegistry) *ControlPlaneServer {
	return &ControlPlaneServer{
		cfg:   cfg,
		repo:  repo,
		nodes: nodes,
	}
}

func (s *ControlPlaneServer) SubmitExperimentBatch(ctx context.Context, req *cpb.SubmitExperimentBatchRequest) (*cpb.SubmitExperimentBatchResponse, error) {
	log.Printf(
		"submit experiment batch rejected: batch_id=%s reason=not_implemented",
		req.GetBatchId(),
	)

	return &cpb.SubmitExperimentBatchResponse{
		Accepted:  false,
		BatchId:   req.GetBatchId(),
		Status:    statusFailed,
		ErrorCode: "not_implemented",
		Message:   "SubmitExperimentBatch will be implemented after codegen NATS client and scheduler",
	}, nil
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
	nodes := s.nodes.List(req.GetStatus())

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
