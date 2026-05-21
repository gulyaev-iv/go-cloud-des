package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	pb "github.com/gulyaev-iv/go-cloud-des/api/dispatcher/v1"
)

type DispatcherServer struct {
	pb.UnimplementedDispatcherServiceServer

	cfg ServeConfig

	tasks chan ExperimentTask

	store *S3Store
	cache *ModelCache

	registry *Registry

	controlPlane *GRPCControlPlaneClient
	reporter     *Reporter

	workersWG sync.WaitGroup
}

func NewDispatcherServer(
	cfg ServeConfig,
	store *S3Store,
	cache *ModelCache,
	registry *Registry,
	controlPlane *GRPCControlPlaneClient,
	reporter *Reporter,
) *DispatcherServer {
	return &DispatcherServer{
		cfg:          cfg,
		tasks:        make(chan ExperimentTask),
		store:        store,
		cache:        cache,
		registry:     registry,
		controlPlane: controlPlane,
		reporter:     reporter,
	}
}

func (s *DispatcherServer) StartExperiment(ctx context.Context, req *pb.StartExperimentRequest) (*pb.StartExperimentResponse, error) {
	if req.GetModelHash() == "" {
		return rejectedStart(req, "bad_request", "empty model_hash"), nil
	}
	if req.GetExperimentId() == "" {
		return rejectedStart(req, "bad_request", "empty experiment_id"), nil
	}
	if len(req.GetModelHash()) != 64 || !isHex(req.GetModelHash()) {
		return rejectedStart(req, "bad_request", "model_hash must be 64 hex characters"), nil
	}
	if req.GetBinary() == nil || req.GetBinary().GetKey() == "" {
		return rejectedStart(req, "bad_request", "empty binary key"), nil
	}
	if req.GetBinary().GetSha256() != "" && (len(req.GetBinary().GetSha256()) != 64 || !isHex(req.GetBinary().GetSha256())) {
		return rejectedStart(req, "bad_request", "binary sha256 must be 64 hex characters"), nil
	}

	record, code, message := s.registry.TryStart(req.GetModelHash(), req.GetExperimentId(), req.GetMemoryLimitBytes())
	if code != "" {
		return rejectedStart(req, code, message), nil
	}

	runCtx := context.Background()
	if req.GetRunTimeoutMs() > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(runCtx, time.Duration(req.GetRunTimeoutMs())*time.Millisecond)
		s.registry.SetCancel(req.GetModelHash(), req.GetExperimentId(), cancel)
	} else {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithCancel(runCtx)
		s.registry.SetCancel(req.GetModelHash(), req.GetExperimentId(), cancel)
	}

	cfg := configFromStartRequest(
		req.GetModelHash(),
		req.GetExperimentId(),
		BinaryArtifact{
			Key:    req.GetBinary().GetKey(),
			SHA256: req.GetBinary().GetSha256(),
		},
		pbSetVarsToDomain(req.GetSetVars()),
		pbStopRuleToDomain(req.GetStopRule()),
		req.GetMetrics(),
		req.GetMetricStep(),
		req.GetStoreMetrics(),
		req.GetMemoryLimitBytes(),
		req.GetRunTimeoutMs(),
		s.baseRunConfig(),
	)

	log.Printf(
		"start experiment accepted: model_hash=%s experiment_id=%s binary_key=%s store_metrics=%t metric_step=%g memory_limit_bytes=%d run_timeout_ms=%d",
		req.GetModelHash(),
		req.GetExperimentId(),
		req.GetBinary().GetKey(),
		req.GetStoreMetrics(),
		req.GetMetricStep(),
		req.GetMemoryLimitBytes(),
		req.GetRunTimeoutMs(),
	)

	select {
	case s.tasks <- ExperimentTask{Ctx: runCtx, Config: cfg}:
		return &pb.StartExperimentResponse{
			Accepted:     true,
			ModelHash:    record.Key.ModelHash,
			ExperimentId: record.Key.ExperimentID,
			Status:       statusStarting,
			Message:      "experiment accepted",
		}, nil

	case <-ctx.Done():
		s.registry.Finish(req.GetModelHash(), req.GetExperimentId(), ExperimentResult{
			ModelHash:        req.GetModelHash(),
			ExperimentID:     req.GetExperimentId(),
			Status:           statusFailed,
			ExitCode:         -1,
			ErrorMessage:     ctx.Err().Error(),
			MemoryLimitBytes: req.GetMemoryLimitBytes(),
		})
		return rejectedStart(req, "request_canceled", ctx.Err().Error()), nil
	}
}

func (s *DispatcherServer) StartWorkers(ctx context.Context) {
	for i := uint64(0); i < s.cfg.Slots; i++ {
		s.workersWG.Add(1)
		go s.worker(ctx, i)
	}
}

func (s *DispatcherServer) WaitWorkers() {
	s.workersWG.Wait()
}

func (s *DispatcherServer) worker(ctx context.Context, workerID uint64) {
	defer s.workersWG.Done()

	log.Printf("worker started: worker_id=%d", workerID)

	for {
		select {
		case <-ctx.Done():
			log.Printf("worker stopped: worker_id=%d", workerID)
			return

		case task := <-s.tasks:
			log.Printf(
				"worker received task: worker_id=%d model_hash=%s experiment_id=%s",
				workerID,
				task.Config.ModelHash,
				task.Config.ExperimentID,
			)

			s.runExperimentTask(workerID, task)
		}
	}
}

func (s *DispatcherServer) runExperimentTask(workerID uint64, task ExperimentTask) {
	s.registry.SetStatus(task.Config.ModelHash, task.Config.ExperimentID, statusRunning)

	result := executeExperiment(task.Ctx, task.Config, s.store, s.cache)

	finalResult := s.registry.Finish(task.Config.ModelHash, task.Config.ExperimentID, result)

	log.Printf(
		"experiment finished: worker_id=%d model_hash=%s experiment_id=%s status=%s finish_reason=%s exit_code=%d stat_count=%d duration_ms=%d error=%q",
		workerID,
		finalResult.ModelHash,
		finalResult.ExperimentID,
		finalResult.Status,
		finalResult.FinishReason,
		finalResult.ExitCode,
		finalResult.StatCount,
		finalResult.DurationMS,
		finalResult.ErrorMessage,
	)

	if finalResult.MetricsArtifact != nil {
		log.Printf(
			"experiment metrics stored: model_hash=%s experiment_id=%s key=%s sha256=%s size=%d",
			finalResult.ModelHash,
			finalResult.ExperimentID,
			finalResult.MetricsArtifact.Key,
			finalResult.MetricsArtifact.SHA256,
			finalResult.MetricsArtifact.Size,
		)
	}

	s.reporter.Submit(finalResult)
}

func (s *DispatcherServer) StopExperiment(ctx context.Context, req *pb.StopExperimentRequest) (*pb.StopExperimentResponse, error) {
	stopped, message := s.registry.Cancel(req.GetModelHash(), req.GetExperimentId())

	log.Printf(
		"stop experiment result: model_hash=%s experiment_id=%s stopped=%t message=%s",
		req.GetModelHash(),
		req.GetExperimentId(),
		stopped,
		message,
	)

	status := statusCanceled
	if !stopped {
		status = statusFailed
	}

	return &pb.StopExperimentResponse{
		Stopped:      stopped,
		ModelHash:    req.GetModelHash(),
		ExperimentId: req.GetExperimentId(),
		Status:       status,
		Message:      message,
	}, nil
}

func (s *DispatcherServer) GetExperiment(ctx context.Context, req *pb.GetExperimentRequest) (*pb.ExperimentStatus, error) {
	result, ok := s.registry.Get(req.GetModelHash(), req.GetExperimentId())
	if !ok {
		return nil, fmt.Errorf("experiment not found: %s/%s", req.GetModelHash(), req.GetExperimentId())
	}

	return experimentResultToPB(result), nil
}

func (s *DispatcherServer) ListExperiments(ctx context.Context, req *pb.ListExperimentsRequest) (*pb.ListExperimentsResponse, error) {
	results := s.registry.List(req.GetStatus())

	response := &pb.ListExperimentsResponse{
		Experiments: make([]*pb.ExperimentStatus, 0, len(results)),
	}

	for _, result := range results {
		response.Experiments = append(response.Experiments, experimentResultToPB(result))
	}

	return response, nil
}

func (s *DispatcherServer) GetNodeStatus(ctx context.Context, req *pb.GetNodeStatusRequest) (*pb.NodeStatus, error) {
	return nodeStatus(s.cfg, s.registry, s.cache), nil
}

func (s *DispatcherServer) baseRunConfig() Config {
	return Config{
		WorkDir: s.cfg.WorkDir,

		CgroupEnabled: s.cfg.CgroupEnabled,
		CgroupRoot:    s.cfg.CgroupRoot,
		CgroupParent:  s.cfg.CgroupParent,

		S3Endpoint:     s.cfg.S3Endpoint,
		S3AccessKey:    s.cfg.S3AccessKey,
		S3SecretKey:    s.cfg.S3SecretKey,
		S3Bucket:       s.cfg.S3Bucket,
		S3UseSSL:       s.cfg.S3UseSSL,
		S3Region:       s.cfg.S3Region,
		S3Prefix:       s.cfg.S3Prefix,
		S3CreateBucket: s.cfg.S3CreateBucket,
	}
}

func rejectedStart(req *pb.StartExperimentRequest, code string, message string) *pb.StartExperimentResponse {
	log.Printf(
		"start experiment rejected: model_hash=%s experiment_id=%s error_code=%s message=%s",
		req.GetModelHash(),
		req.GetExperimentId(),
		code,
		message,
	)

	return &pb.StartExperimentResponse{
		Accepted:     false,
		ModelHash:    req.GetModelHash(),
		ExperimentId: req.GetExperimentId(),
		Status:       statusFailed,
		ErrorCode:    code,
		Message:      message,
	}
}

func pbSetVarsToDomain(values []*pb.SetVar) []SetVar {
	result := make([]SetVar, 0, len(values))

	for _, v := range values {
		if v == nil {
			continue
		}

		result = append(result, SetVar{
			Name:  v.GetName(),
			Value: v.GetValue(),
		})
	}

	return result
}

func pbStopRuleToDomain(value *pb.StopRule) *StopRule {
	if value == nil {
		return nil
	}

	left := strings.TrimSpace(value.GetLeft())
	op := strings.TrimSpace(value.GetOp())
	right := strings.TrimSpace(value.GetRight())

	if left == "" && op == "" && right == "" {
		return nil
	}

	return &StopRule{
		Left:  left,
		Op:    op,
		Right: right,
	}
}

func experimentResultToPB(result ExperimentResult) *pb.ExperimentStatus {
	return &pb.ExperimentStatus{
		ModelHash:    result.ModelHash,
		ExperimentId: result.ExperimentID,
		Status:       result.Status,

		FinishReason: result.FinishReason,
		ExitCode:     int32(result.ExitCode),
		ErrorMessage: result.ErrorMessage,

		FinalMetrics: result.FinalMetrics,

		StatCount:       result.StatCount,
		MetricsFileRows: result.MetricsFileRows,
		MetricsArtifact: artifactRefToPB(result.MetricsArtifact),

		StartedAtUnixMs:  result.StartedAtUnixMS,
		FinishedAtUnixMs: result.FinishedAtUnixMS,
		DurationMs:       result.DurationMS,

		MemoryLimitBytes: result.MemoryLimitBytes,
		PeakMemoryBytes:  result.PeakMemoryBytes,

		StderrTail: result.StderrTail,
	}
}

func artifactRefToPB(ref *ArtifactRef) *pb.ArtifactRef {
	if ref == nil {
		return nil
	}

	return &pb.ArtifactRef{
		Key:         ref.Key,
		Sha256:      ref.SHA256,
		Size:        ref.Size,
		ContentType: ref.ContentType,
	}
}
