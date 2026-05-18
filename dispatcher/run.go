package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func executeExperiment(ctx context.Context, cfg Config, store *S3Store, cache *ModelCache) ExperimentResult {
	result := ExperimentResult{
		ExperimentID:     cfg.ExperimentID,
		ModelHash:        cfg.ModelHash,
		Status:           statusFailed,
		ExitCode:         -1,
		FinalMetrics:     map[string]string{},
		MemoryLimitBytes: cfg.MemoryLimitBytes,
		PeakMemoryBytes:  0,
	}

	setVars, err := parseVarOverrides(cfg.SetFlags)
	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}

	stopRule, err := parseStopRule(cfg.StopRuleRaw)
	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}

	runtimeMetrics := normalizeMetrics(cfg.MetricsRaw)

	experimentDir := filepath.Join(cfg.WorkDir, "experiments", safePathName(cfg.ModelHash), safePathName(cfg.ExperimentID))
	if err := os.MkdirAll(experimentDir, 0o755); err != nil {
		result.ErrorMessage = err.Error()
		return result
	}

	binaryKey, err := normalizeS3ObjectKey(cfg.BinaryKey)
	if err != nil {
		result.ErrorMessage = err.Error()
		return result
	}

	cachedModel, err := cache.GetOrDownload(ctx, cfg.ModelHash, binaryKey, cfg.BinarySHA256)
	if err != nil {
		result.ErrorMessage = "prepare binary: " + err.Error()
		return result
	}

	var trace *CSVTraceWriter
	if cfg.StoreMetrics {
		tracePath := filepath.Join(experimentDir, "metrics.csv")
		trace, err = NewCSVTraceWriter(tracePath, runtimeMetrics)
		if err != nil {
			result.ErrorMessage = "create metrics csv: " + err.Error()
			return result
		}
	}

	runResult := RunModelProcess(ctx, ModelRunConfig{
		ExperimentID:     cfg.ExperimentID,
		ModelHash:        cfg.ModelHash,
		BinaryPath:       cachedModel.LocalPath,
		SetVars:          setVars,
		StopRule:         stopRule,
		Metrics:          runtimeMetrics,
		MetricStep:       cfg.MetricStep,
		Trace:            trace,
		MemoryLimitBytes: cfg.MemoryLimitBytes,
	})

	result.Status = runResult.Status
	result.FinishReason = runResult.FinishReason
	result.ExitCode = runResult.ExitCode
	result.ErrorMessage = runResult.ErrorMessage
	result.FinalMetrics = runResult.FinalMetrics
	result.StatCount = runResult.StatCount
	result.StartedAtUnixMS = runResult.StartedAtUnixMS
	result.FinishedAtUnixMS = runResult.FinishedAtUnixMS
	result.DurationMS = runResult.DurationMS
	result.MemoryLimitBytes = runResult.MemoryLimitBytes
	result.PeakMemoryBytes = runResult.PeakMemoryBytes
	result.StderrTail = runResult.StderrTail

	if trace != nil {
		closeErr := trace.Close()
		result.MetricsFileRows = trace.Rows()

		if closeErr != nil {
			appendResultError(&result, "close metrics csv: "+closeErr.Error())
		} else {
			metricsKey := fmt.Sprintf("models/%s/experiments/%s/metrics.csv", cfg.ModelHash, cfg.ExperimentID)

			ref, uploadErr := store.PutFile(ctx, metricsKey, trace.Path(), "text/csv; charset=utf-8")
			if uploadErr != nil {
				appendResultError(&result, "upload metrics csv: "+uploadErr.Error())
			} else {
				result.MetricsArtifact = ref

				log.Printf(
					"experiment metrics stored: model_hash=%s experiment_id=%s key=%s rows=%d size=%d",
					cfg.ModelHash,
					cfg.ExperimentID,
					ref.Key,
					result.MetricsFileRows,
					ref.Size,
				)
			}
		}
	}

	return result
}

func configFromStartRequest(
	reqModelHash string,
	reqExperimentID string,
	binary BinaryArtifact,
	setVars []SetVar,
	stopRule *StopRule,
	metrics []string,
	metricStep float64,
	storeMetrics bool,
	memoryLimitBytes uint64,
	runTimeoutMS uint64,
	base Config,
) Config {
	cfg := base

	cfg.ExperimentID = reqExperimentID
	cfg.ModelHash = reqModelHash
	cfg.BinaryKey = binary.Key
	cfg.BinarySHA256 = binary.SHA256

	cfg.SetFlags = nil
	for _, v := range setVars {
		cfg.SetFlags = append(cfg.SetFlags, v.Name+"="+v.Value)
	}

	cfg.StopRuleRaw = ""
	if stopRule != nil {
		cfg.StopRuleRaw = strings.TrimSpace(stopRule.Left + " " + stopRule.Op + " " + stopRule.Right)
	}

	cfg.MetricsRaw = strings.Join(metrics, ";")
	cfg.MetricStep = metricStep
	cfg.StoreMetrics = storeMetrics

	cfg.MemoryLimitBytes = memoryLimitBytes

	if runTimeoutMS > 0 {
		cfg.RunTimeout = time.Duration(runTimeoutMS) * time.Millisecond
	} else {
		cfg.RunTimeout = 0
	}

	return cfg
}

func appendResultError(result *ExperimentResult, message string) {
	if result.ErrorMessage == "" {
		result.ErrorMessage = message
	} else {
		result.ErrorMessage += "; " + message
	}
	result.Status = statusFailed
}
