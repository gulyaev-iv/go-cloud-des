package main

import "context"

const (
	statusPending  = "PENDING"
	statusStarting = "STARTING"
	statusRunning  = "RUNNING"
	statusFinished = "FINISHED"
	statusFailed   = "FAILED"
	statusCanceled = "CANCELED"

	metricClock = "Simulation.clock"
)

type BinaryArtifact struct {
	Key    string
	SHA256 string
}

type SetVar struct {
	Name  string
	Value string
}

type StopRule struct {
	Left  string
	Op    string
	Right string
}

type ExperimentTask struct {
	Ctx    context.Context
	Config Config
}

type ArtifactRef struct {
	Key         string `json:"key"`
	SHA256      string `json:"sha256"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}

type ExperimentResult struct {
	ExperimentID string            `json:"experiment_id"`
	ModelHash    string            `json:"model_hash"`
	Status       string            `json:"status"`
	FinishReason string            `json:"finish_reason,omitempty"`
	ExitCode     int               `json:"exit_code"`
	ErrorMessage string            `json:"error_message,omitempty"`
	FinalMetrics map[string]string `json:"final_metrics,omitempty"`

	StatCount       uint64       `json:"stat_count"`
	MetricsFileRows uint64       `json:"metrics_file_rows,omitempty"`
	MetricsArtifact *ArtifactRef `json:"metrics_artifact,omitempty"`

	StartedAtUnixMS  int64 `json:"started_at_unix_ms,omitempty"`
	FinishedAtUnixMS int64 `json:"finished_at_unix_ms,omitempty"`
	DurationMS       int64 `json:"duration_ms,omitempty"`

	MemoryLimitBytes uint64 `json:"memory_limit_bytes,omitempty"`
	PeakMemoryBytes  uint64 `json:"peak_memory_bytes,omitempty"`

	StderrTail string `json:"stderr_tail,omitempty"`
}

type ExperimentKey struct {
	ModelHash    string
	ExperimentID string
}

func experimentKey(modelHash string, experimentID string) ExperimentKey {
	return ExperimentKey{
		ModelHash:    modelHash,
		ExperimentID: experimentID,
	}
}

func isActiveStatus(status string) bool {
	switch status {
	case statusPending, statusStarting, statusRunning:
		return true
	default:
		return false
	}
}
