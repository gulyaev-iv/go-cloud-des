package main

import "time"

type ProcessOutput struct {
	Result  TaskResult
	Metrics TaskMetrics
}

type TaskMetrics struct {
	RequestID string `json:"request_id"`
	ModelHash string `json:"model_hash,omitempty"`

	Mode   string `json:"mode,omitempty"`
	GOOS   string `json:"goos,omitempty"`
	GOARCH string `json:"goarch,omitempty"`

	OK        bool   `json:"ok"`
	Stage     string `json:"stage,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`

	ArtifactStore string `json:"artifact_store,omitempty"`

	GenerateMS      float64 `json:"generate_ms"`
	BuildMS         float64 `json:"build_ms"`
	StoreSourceMS   float64 `json:"store_source_ms"`
	StoreBinaryMS   float64 `json:"store_binary_ms"`
	StoreBuildLogMS float64 `json:"store_build_log_ms"`
	StoreTotalMS    float64 `json:"store_total_ms"`
	TotalMS         float64 `json:"total_ms"`

	SourceSize int64 `json:"source_size,omitempty"`
	BinarySize int64 `json:"binary_size,omitempty"`
}

func durationMS(d time.Duration) float64 {
	return float64(d.Nanoseconds()) / 1_000_000
}

func completeTaskMetrics(metrics *TaskMetrics, totalStart time.Time, result TaskResult) {
	metrics.RequestID = result.RequestID
	metrics.ModelHash = result.ModelHash
	metrics.Mode = result.Mode
	metrics.GOOS = result.GOOS
	metrics.GOARCH = result.GOARCH

	metrics.OK = result.OK
	metrics.Stage = result.Stage
	metrics.ErrorCode = result.ErrorCode

	metrics.StoreTotalMS = metrics.StoreSourceMS + metrics.StoreBinaryMS + metrics.StoreBuildLogMS
	metrics.TotalMS = durationMS(time.Since(totalStart))

	if result.Source != nil {
		metrics.SourceSize = result.Source.Size
	}

	if result.Binary != nil {
		metrics.BinarySize = result.Binary.Size
	}
}
