package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"

	cpb "github.com/gulyaev-iv/go-cloud-des/api/controlplane/v1"
	dpb "github.com/gulyaev-iv/go-cloud-des/api/dispatcher/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *ControlPlaneServer) SubmitExperimentBatch(ctx context.Context, req *cpb.SubmitExperimentBatchRequest) (*cpb.SubmitExperimentBatchResponse, error) {
	batchID, experiments, err := normalizeSubmitRequest(req)
	if err != nil {
		return &cpb.SubmitExperimentBatchResponse{
			Accepted:  false,
			BatchId:   req.GetBatchId(),
			Status:    statusFailed,
			ErrorCode: "bad_request",
			Message:   err.Error(),
		}, nil
	}

	modelHash := hashDSL(req.GetDsl())

	storeSource := true
	storeBinary := true
	buildTimeoutSec := req.GetBuildTimeoutSec()
	if buildTimeoutSec <= 0 {
		buildTimeoutSec = 120
	}

	input := CreateExperimentBatchInput{
		BatchID:         batchID,
		ModelHash:       modelHash,
		DSL:             req.GetDsl(),
		SelectedGOOS:    s.cfg.DefaultGOOS,
		SelectedGOARCH:  s.cfg.DefaultGOARCH,
		StoreSource:     storeSource,
		StoreBinary:     storeBinary,
		BuildTimeoutSec: buildTimeoutSec,
		Common:          req.GetCommon(),
		Experiments:     experiments,
	}

	if err := s.repo.CreateExperimentBatch(ctx, input); err != nil {
		log.Printf(
			"submit experiment batch failed: batch_id=%s model_hash=%s error=%v",
			batchID,
			modelHash,
			err,
		)

		return nil, status.Error(codes.Internal, err.Error())
	}

	batchStatus := statusSubmitted
	var errorCode string
	var message string

	artifact, ok, err := s.repo.GetReadyModelArtifact(ctx, modelHash, s.cfg.DefaultGOOS, s.cfg.DefaultGOARCH)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	if ok {
		batchStatus = statusReady
		if err := s.repo.UpdateBatchStatus(ctx, batchID, batchStatus, "", ""); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		log.Printf(
			"model artifact reused: batch_id=%s model_hash=%s goos=%s goarch=%s binary_key=%s",
			batchID,
			modelHash,
			s.cfg.DefaultGOOS,
			s.cfg.DefaultGOARCH,
			artifact.Binary.Key,
		)
	} else {
		batchStatus, errorCode, message, err = s.buildModelArtifact(ctx, batchID, modelHash, req.GetDsl(), buildTimeoutSec)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	if batchStatus == statusReady && s.scheduler != nil {
		records, err := s.repo.ListSchedulableExperimentsByBatch(ctx, batchID)
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}

		s.scheduler.Enqueue(records)
	}

	submitted := make([]*cpb.SubmittedExperiment, 0, len(experiments))
	for _, experiment := range experiments {
		submitted = append(submitted, &cpb.SubmittedExperiment{
			ModelHash:    modelHash,
			ExperimentId: experiment.ExperimentID,
			Status:       statusPending,
		})
	}

	log.Printf(
		"experiment batch submitted: batch_id=%s model_hash=%s experiments=%d selected_goos=%s selected_goarch=%s",
		batchID,
		modelHash,
		len(experiments),
		s.cfg.DefaultGOOS,
		s.cfg.DefaultGOARCH,
	)

	return &cpb.SubmitExperimentBatchResponse{
		Accepted:       batchStatus != statusFailed,
		BatchId:        batchID,
		ModelHash:      modelHash,
		Status:         batchStatus,
		SelectedGoos:   s.cfg.DefaultGOOS,
		SelectedGoarch: s.cfg.DefaultGOARCH,
		ErrorCode:      errorCode,
		Message:        message,
		Experiments:    submitted,
	}, nil
}

type NormalizedExperiment struct {
	ExperimentID string
	SetVars      []*dpb.SetVar
}

func normalizeSubmitRequest(req *cpb.SubmitExperimentBatchRequest) (string, []*NormalizedExperiment, error) {
	if req == nil {
		return "", nil, fmt.Errorf("empty request")
	}

	if strings.TrimSpace(req.GetDsl()) == "" {
		return "", nil, fmt.Errorf("empty dsl")
	}

	common := req.GetCommon()
	if common == nil {
		return "", nil, fmt.Errorf("empty common config")
	}

	if err := validateStopRule(common.GetStopRule()); err != nil {
		return "", nil, err
	}

	if len(req.GetExperiments()) == 0 {
		return "", nil, fmt.Errorf("empty experiments")
	}

	batchID := strings.TrimSpace(req.GetBatchId())
	if batchID == "" {
		batchID = "batch-" + randomHex(16)
	}

	experiments := make([]*NormalizedExperiment, 0, len(req.GetExperiments()))

	for i, experiment := range req.GetExperiments() {
		if experiment == nil {
			return "", nil, fmt.Errorf("experiment %d is empty", i)
		}

		normalized := &NormalizedExperiment{
			ExperimentID: fmt.Sprintf("%s-exp-%06d", batchID, i+1),
			SetVars:      experiment.GetSetVars(),
		}

		for _, setVar := range normalized.SetVars {
			if setVar == nil {
				return "", nil, fmt.Errorf("experiment %s has empty set_var", normalized.ExperimentID)
			}
			if strings.TrimSpace(setVar.GetName()) == "" {
				return "", nil, fmt.Errorf("experiment %s has set_var with empty name", normalized.ExperimentID)
			}
			if strings.TrimSpace(setVar.GetValue()) == "" {
				return "", nil, fmt.Errorf("experiment %s has set_var %s with empty value", normalized.ExperimentID, setVar.GetName())
			}
		}

		experiments = append(experiments, normalized)
	}

	return batchID, experiments, nil
}

func validateStopRule(rule *dpb.StopRule) error {
	if rule == nil {
		return fmt.Errorf("empty stop_rule")
	}

	left := strings.TrimSpace(rule.GetLeft())
	op := strings.TrimSpace(rule.GetOp())
	right := strings.TrimSpace(rule.GetRight())

	if left == "" {
		return fmt.Errorf("empty stop_rule.left")
	}
	if op == "" {
		return fmt.Errorf("empty stop_rule.op")
	}
	if right == "" {
		return fmt.Errorf("empty stop_rule.right")
	}

	switch op {
	case "==", "!=", ">", ">=", "<", "<=":
		return nil
	default:
		return fmt.Errorf("unsupported stop_rule.op %q", op)
	}
}

func hashDSL(dsl string) string {
	sum := sha256.Sum256([]byte(dsl))
	return hex.EncodeToString(sum[:])
}

func randomHex(bytesN int) string {
	buf := make([]byte, bytesN)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}

	return hex.EncodeToString(buf)
}

func (s *ControlPlaneServer) buildModelArtifact(ctx context.Context, batchID string, modelHash string, dsl string, buildTimeoutSec int32) (string, string, string, error) {
	if err := s.repo.MarkModelArtifactBuilding(ctx, modelHash, s.cfg.DefaultGOOS, s.cfg.DefaultGOARCH); err != nil {
		return statusFailed, "store_artifact_error", err.Error(), err
	}

	if err := s.repo.UpdateBatchStatus(ctx, batchID, statusBuilding, "", ""); err != nil {
		return statusFailed, "update_batch_error", err.Error(), err
	}

	storeSource := true
	storeBinary := true

	task := CodegenTask{
		RequestID:       "codegen-" + randomHex(16),
		ReplySubject:    "",
		ModelHash:       modelHash,
		DSL:             dsl,
		Mode:            codegenModeBuild,
		GOOS:            s.cfg.DefaultGOOS,
		GOARCH:          s.cfg.DefaultGOARCH,
		StoreSource:     &storeSource,
		StoreBinary:     &storeBinary,
		BuildTimeoutSec: int(buildTimeoutSec),
	}

	result, err := s.codegen.BuildModel(ctx, task)
	if err != nil {
		if saveErr := s.repo.UpdateBatchStatus(ctx, batchID, statusFailed, "codegen_request_error", err.Error()); saveErr != nil {
			return statusFailed, "codegen_request_error", err.Error(), saveErr
		}

		return statusFailed, "codegen_request_error", err.Error(), nil
	}

	if err := s.repo.SaveCodegenResult(ctx, task, result); err != nil {
		return statusFailed, "store_artifact_result_error", err.Error(), err
	}

	if !result.OK {
		errorCode := result.ErrorCode
		if errorCode == "" {
			errorCode = "codegen_error"
		}

		message := result.Message
		if message == "" {
			message = "codegen task failed"
		}

		if err := s.repo.UpdateBatchStatus(ctx, batchID, statusFailed, errorCode, message); err != nil {
			return statusFailed, errorCode, message, err
		}

		return statusFailed, errorCode, message, nil
	}

	if result.Binary == nil || result.Binary.Key == "" || result.Binary.SHA256 == "" {
		message := "codegen returned no binary artifact"

		if err := s.repo.UpdateBatchStatus(ctx, batchID, statusFailed, "empty_binary_artifact", message); err != nil {
			return statusFailed, "empty_binary_artifact", message, err
		}

		return statusFailed, "empty_binary_artifact", message, nil
	}

	if err := s.repo.UpdateBatchStatus(ctx, batchID, statusReady, "", ""); err != nil {
		return statusFailed, "update_batch_error", err.Error(), err
	}

	log.Printf(
		"model artifact built: batch_id=%s model_hash=%s goos=%s goarch=%s binary_key=%s",
		batchID,
		modelHash,
		s.cfg.DefaultGOOS,
		s.cfg.DefaultGOARCH,
		result.Binary.Key,
	)

	return statusReady, "", "", nil
}
