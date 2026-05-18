package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	cpb "github.com/gulyaev-iv/go-cloud-des/api/controlplane/v1"
	dpb "github.com/gulyaev-iv/go-cloud-des/api/dispatcher/v1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const maxInt64Uint = uint64(1<<63 - 1)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) UpdateExperimentResult(ctx context.Context, nodeID string, result *dpb.ExperimentStatus) (string, bool, error) {
	if result == nil {
		return "", false, fmt.Errorf("empty experiment result")
	}

	finalMetrics := result.GetFinalMetrics()
	if finalMetrics == nil {
		finalMetrics = map[string]string{}
	}

	finalMetricsJSON, err := json.Marshal(finalMetrics)
	if err != nil {
		return "", false, err
	}

	artifact := result.GetMetricsArtifact()

	var artifactKey any
	var artifactSHA any
	var artifactSize any
	var artifactContentType any

	if artifact != nil {
		artifactKey = nilIfEmpty(artifact.GetKey())
		artifactSHA = nilIfEmpty(artifact.GetSha256())
		artifactSize = artifact.GetSize()
		artifactContentType = nilIfEmpty(artifact.GetContentType())
	}

	var batchID string

	err = r.db.QueryRow(ctx, `
UPDATE experiments
SET
    status = $3,
    dispatcher_node_id = $4,

    finish_reason = $5,
    exit_code = $6,
    error_message = $7,

    final_metrics = $8,

    stat_count = $9,
    metrics_file_rows = $10,

    metrics_artifact_key = $11,
    metrics_artifact_sha256 = $12,
    metrics_artifact_size = $13,
    metrics_artifact_content_type = $14,

    started_at_unix_ms = $15,
    finished_at_unix_ms = $16,
    duration_ms = $17,

    memory_limit_bytes = $18,
    peak_memory_bytes = $19,

    stderr_tail = $20,

    updated_at = now()
WHERE model_hash = $1 AND experiment_id = $2
RETURNING batch_id
`,
		result.GetModelHash(),
		result.GetExperimentId(),
		result.GetStatus(),
		nilIfEmpty(nodeID),
		nilIfEmpty(result.GetFinishReason()),
		result.GetExitCode(),
		nilIfEmpty(result.GetErrorMessage()),
		finalMetricsJSON,
		u64ToI64(result.GetStatCount()),
		u64ToI64(result.GetMetricsFileRows()),
		artifactKey,
		artifactSHA,
		artifactSize,
		artifactContentType,
		nilIfZeroI64(result.GetStartedAtUnixMs()),
		nilIfZeroI64(result.GetFinishedAtUnixMs()),
		nilIfZeroI64(result.GetDurationMs()),
		u64ToI64(result.GetMemoryLimitBytes()),
		u64ToI64(result.GetPeakMemoryBytes()),
		nilIfEmpty(result.GetStderrTail()),
	).Scan(&batchID)

	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}

	if err := r.RefreshBatchStatus(ctx, batchID); err != nil {
		return batchID, true, err
	}

	return batchID, true, nil
}

func (r *Repository) RefreshBatchStatus(ctx context.Context, batchID string) error {
	var total int
	var pending int
	var running int
	var finished int
	var failed int
	var canceled int

	err := r.db.QueryRow(ctx, `
SELECT
    count(*)::int,
    count(*) FILTER (WHERE status = 'PENDING' OR status = 'STARTING')::int,
    count(*) FILTER (WHERE status = 'RUNNING')::int,
    count(*) FILTER (WHERE status = 'FINISHED')::int,
    count(*) FILTER (WHERE status = 'FAILED')::int,
    count(*) FILTER (WHERE status = 'CANCELED')::int
FROM experiments
WHERE batch_id = $1
`, batchID).Scan(&total, &pending, &running, &finished, &failed, &canceled)
	if err != nil {
		return err
	}

	batchStatus := statusRunning

	if total == 0 {
		batchStatus = statusSubmitted
	} else if failed > 0 && finished+failed+canceled == total {
		batchStatus = statusFailed
	} else if canceled > 0 && finished+canceled == total {
		batchStatus = statusCanceled
	} else if finished == total {
		batchStatus = statusFinished
	} else if pending > 0 || running > 0 {
		batchStatus = statusRunning
	}

	_, err = r.db.Exec(ctx, `
UPDATE experiment_batches
SET status = $2, updated_at = now()
WHERE batch_id = $1
`, batchID, batchStatus)

	return err
}

func (r *Repository) GetExperiment(ctx context.Context, modelHash string, experimentID string) (*dpb.ExperimentStatus, bool, error) {
	row := r.db.QueryRow(ctx, `
SELECT
    model_hash,
    experiment_id,
    status,

    finish_reason,
    exit_code,
    error_message,

    final_metrics,

    stat_count,
    metrics_file_rows,

    metrics_artifact_key,
    metrics_artifact_sha256,
    metrics_artifact_size,
    metrics_artifact_content_type,

    started_at_unix_ms,
    finished_at_unix_ms,
    duration_ms,

    memory_limit_bytes,
    peak_memory_bytes,

    stderr_tail
FROM experiments
WHERE model_hash = $1 AND experiment_id = $2
`, modelHash, experimentID)

	result, err := scanExperiment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	return result, true, nil
}

func (r *Repository) GetExperimentBatch(ctx context.Context, batchID string) (*cpb.ExperimentBatchStatus, bool, error) {
	var row struct {
		BatchID        string
		ModelHash      string
		Status         string
		SelectedGOOS   string
		SelectedGOARCH string

		ErrorCode sql.NullString
		Message   sql.NullString

		ArtifactStatus sql.NullString

		SourceKey         sql.NullString
		SourceSHA256      sql.NullString
		SourceSize        sql.NullInt64
		SourceContentType sql.NullString

		BinaryKey         sql.NullString
		BinarySHA256      sql.NullString
		BinarySize        sql.NullInt64
		BinaryContentType sql.NullString

		BuildLogKey         sql.NullString
		BuildLogSHA256      sql.NullString
		BuildLogSize        sql.NullInt64
		BuildLogContentType sql.NullString

		ArtifactErrorCode sql.NullString
		ArtifactMessage   sql.NullString
	}

	err := r.db.QueryRow(ctx, `
SELECT
    b.batch_id,
    b.model_hash,
    b.status,
    b.selected_goos,
    b.selected_goarch,
    b.error_code,
    b.message,

    a.status,

    a.source_key,
    a.source_sha256,
    a.source_size,
    a.source_content_type,

    a.binary_key,
    a.binary_sha256,
    a.binary_size,
    a.binary_content_type,

    a.build_log_key,
    a.build_log_sha256,
    a.build_log_size,
    a.build_log_content_type,

    a.error_code,
    a.message
FROM experiment_batches b
LEFT JOIN model_artifacts a
    ON a.model_hash = b.model_hash
    AND a.goos = b.selected_goos
    AND a.goarch = b.selected_goarch
WHERE b.batch_id = $1
`, batchID).Scan(
		&row.BatchID,
		&row.ModelHash,
		&row.Status,
		&row.SelectedGOOS,
		&row.SelectedGOARCH,
		&row.ErrorCode,
		&row.Message,
		&row.ArtifactStatus,
		&row.SourceKey,
		&row.SourceSHA256,
		&row.SourceSize,
		&row.SourceContentType,
		&row.BinaryKey,
		&row.BinarySHA256,
		&row.BinarySize,
		&row.BinaryContentType,
		&row.BuildLogKey,
		&row.BuildLogSHA256,
		&row.BuildLogSize,
		&row.BuildLogContentType,
		&row.ArtifactErrorCode,
		&row.ArtifactMessage,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	experiments, err := r.ListExperimentsByBatch(ctx, batchID)
	if err != nil {
		return nil, false, err
	}

	status := &cpb.ExperimentBatchStatus{
		BatchId:        row.BatchID,
		ModelHash:      row.ModelHash,
		Status:         row.Status,
		SelectedGoos:   row.SelectedGOOS,
		SelectedGoarch: row.SelectedGOARCH,
		ErrorCode:      nullString(row.ErrorCode),
		Message:        nullString(row.Message),
		Experiments:    experiments,
	}

	status.Artifact = &cpb.ModelArtifact{
		ModelHash: row.ModelHash,
		Goos:      row.SelectedGOOS,
		Goarch:    row.SelectedGOARCH,
		Status:    artifactStatusMissing,
	}

	if row.ArtifactStatus.Valid {
		status.Artifact.Status = row.ArtifactStatus.String
		status.Artifact.Source = artifactFromNulls(row.SourceKey, row.SourceSHA256, row.SourceSize, row.SourceContentType)
		status.Artifact.Binary = artifactFromNulls(row.BinaryKey, row.BinarySHA256, row.BinarySize, row.BinaryContentType)
		status.Artifact.BuildLog = artifactFromNulls(row.BuildLogKey, row.BuildLogSHA256, row.BuildLogSize, row.BuildLogContentType)
		status.Artifact.ErrorCode = nullString(row.ArtifactErrorCode)
		status.Artifact.Message = nullString(row.ArtifactMessage)
	}

	for _, experiment := range experiments {
		status.TotalExperiments++

		switch experiment.GetStatus() {
		case "PENDING", "STARTING":
			status.PendingExperiments++
		case "RUNNING":
			status.RunningExperiments++
		case "FINISHED":
			status.FinishedExperiments++
		case "FAILED":
			status.FailedExperiments++
		case "CANCELED":
			status.CanceledExperiments++
		}
	}

	return status, true, nil
}

func (r *Repository) ListExperimentsByBatch(ctx context.Context, batchID string) ([]*dpb.ExperimentStatus, error) {
	rows, err := r.db.Query(ctx, `
SELECT
    model_hash,
    experiment_id,
    status,

    finish_reason,
    exit_code,
    error_message,

    final_metrics,

    stat_count,
    metrics_file_rows,

    metrics_artifact_key,
    metrics_artifact_sha256,
    metrics_artifact_size,
    metrics_artifact_content_type,

    started_at_unix_ms,
    finished_at_unix_ms,
    duration_ms,

    memory_limit_bytes,
    peak_memory_bytes,

    stderr_tail
FROM experiments
WHERE batch_id = $1
ORDER BY experiment_id
`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []*dpb.ExperimentStatus{}

	for rows.Next() {
		experiment, err := scanExperiment(rows)
		if err != nil {
			return nil, err
		}

		result = append(result, experiment)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

type experimentScanner interface {
	Scan(dest ...any) error
}

func scanExperiment(scanner experimentScanner) (*dpb.ExperimentStatus, error) {
	var modelHash string
	var experimentID string
	var status string

	var finishReason sql.NullString
	var exitCode int32
	var errorMessage sql.NullString

	var finalMetricsBytes []byte

	var statCount int64
	var metricsFileRows int64

	var artifactKey sql.NullString
	var artifactSHA256 sql.NullString
	var artifactSize sql.NullInt64
	var artifactContentType sql.NullString

	var startedAt sql.NullInt64
	var finishedAt sql.NullInt64
	var durationMS sql.NullInt64

	var memoryLimitBytes int64
	var peakMemoryBytes int64

	var stderrTail sql.NullString

	err := scanner.Scan(
		&modelHash,
		&experimentID,
		&status,
		&finishReason,
		&exitCode,
		&errorMessage,
		&finalMetricsBytes,
		&statCount,
		&metricsFileRows,
		&artifactKey,
		&artifactSHA256,
		&artifactSize,
		&artifactContentType,
		&startedAt,
		&finishedAt,
		&durationMS,
		&memoryLimitBytes,
		&peakMemoryBytes,
		&stderrTail,
	)
	if err != nil {
		return nil, err
	}

	finalMetrics := map[string]string{}
	if len(finalMetricsBytes) > 0 {
		if err := json.Unmarshal(finalMetricsBytes, &finalMetrics); err != nil {
			return nil, err
		}
	}

	return &dpb.ExperimentStatus{
		ModelHash:        modelHash,
		ExperimentId:     experimentID,
		Status:           status,
		FinishReason:     nullString(finishReason),
		ExitCode:         exitCode,
		ErrorMessage:     nullString(errorMessage),
		FinalMetrics:     finalMetrics,
		StatCount:        int64ToU64(statCount),
		MetricsFileRows:  int64ToU64(metricsFileRows),
		MetricsArtifact:  artifactFromNulls(artifactKey, artifactSHA256, artifactSize, artifactContentType),
		StartedAtUnixMs:  nullInt64Value(startedAt),
		FinishedAtUnixMs: nullInt64Value(finishedAt),
		DurationMs:       nullInt64Value(durationMS),
		MemoryLimitBytes: int64ToU64(memoryLimitBytes),
		PeakMemoryBytes:  int64ToU64(peakMemoryBytes),
		StderrTail:       nullString(stderrTail),
	}, nil
}

func artifactFromNulls(key sql.NullString, sha256 sql.NullString, size sql.NullInt64, contentType sql.NullString) *dpb.ArtifactRef {
	if !key.Valid || key.String == "" {
		return nil
	}

	return &dpb.ArtifactRef{
		Key:         key.String,
		Sha256:      nullString(sha256),
		Size:        nullInt64Value(size),
		ContentType: nullString(contentType),
	}
}

func nullString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}

	return value.String
}

func nullInt64Value(value sql.NullInt64) int64 {
	if !value.Valid {
		return 0
	}

	return value.Int64
}

func nilIfEmpty(value string) any {
	if value == "" {
		return nil
	}

	return value
}

func nilIfZeroI64(value int64) any {
	if value == 0 {
		return nil
	}

	return value
}

func u64ToI64(value uint64) int64 {
	if value > maxInt64Uint {
		return int64(maxInt64Uint)
	}

	return int64(value)
}

func int64ToU64(value int64) uint64 {
	if value < 0 {
		return 0
	}

	return uint64(value)
}
