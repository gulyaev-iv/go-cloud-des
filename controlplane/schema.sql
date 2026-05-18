CREATE TABLE IF NOT EXISTS models (
    model_hash CHAR(64) PRIMARY KEY,
    dsl TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS model_artifacts (
    model_hash CHAR(64) NOT NULL REFERENCES models(model_hash) ON DELETE CASCADE,

    goos TEXT NOT NULL,
    goarch TEXT NOT NULL,

    status TEXT NOT NULL,

    source_key TEXT,
    source_sha256 CHAR(64),
    source_size BIGINT,
    source_content_type TEXT,

    binary_key TEXT,
    binary_sha256 CHAR(64),
    binary_size BIGINT,
    binary_content_type TEXT,

    build_log_key TEXT,
    build_log_sha256 CHAR(64),
    build_log_size BIGINT,
    build_log_content_type TEXT,

    error_code TEXT,
    message TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (model_hash, goos, goarch)
);

CREATE TABLE IF NOT EXISTS experiment_batches (
    batch_id TEXT PRIMARY KEY,

    model_hash CHAR(64) NOT NULL REFERENCES models(model_hash) ON DELETE RESTRICT,

    status TEXT NOT NULL,

    selected_goos TEXT NOT NULL,
    selected_goarch TEXT NOT NULL,

    store_source BOOLEAN NOT NULL DEFAULT true,
    store_binary BOOLEAN NOT NULL DEFAULT true,
    build_timeout_sec INTEGER NOT NULL DEFAULT 120,

    stop_rule JSONB NOT NULL,
    metrics JSONB NOT NULL,
    metric_step DOUBLE PRECISION NOT NULL DEFAULT 0,
    store_metrics BOOLEAN NOT NULL DEFAULT false,

    memory_limit_bytes BIGINT NOT NULL DEFAULT 0,
    run_timeout_ms BIGINT NOT NULL DEFAULT 0,

    error_code TEXT,
    message TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS experiments (
    model_hash CHAR(64) NOT NULL REFERENCES models(model_hash) ON DELETE RESTRICT,
    experiment_id TEXT NOT NULL,

    batch_id TEXT NOT NULL REFERENCES experiment_batches(batch_id) ON DELETE CASCADE,

    status TEXT NOT NULL,

    dispatcher_node_id TEXT,

    set_vars JSONB NOT NULL DEFAULT '[]'::jsonb,

    finish_reason TEXT,
    exit_code INTEGER NOT NULL DEFAULT -1,
    error_message TEXT,

    final_metrics JSONB NOT NULL DEFAULT '{}'::jsonb,

    stat_count BIGINT NOT NULL DEFAULT 0,
    metrics_file_rows BIGINT NOT NULL DEFAULT 0,

    metrics_artifact_key TEXT,
    metrics_artifact_sha256 CHAR(64),
    metrics_artifact_size BIGINT,
    metrics_artifact_content_type TEXT,

    started_at_unix_ms BIGINT,
    finished_at_unix_ms BIGINT,
    duration_ms BIGINT,

    memory_limit_bytes BIGINT NOT NULL DEFAULT 0,
    peak_memory_bytes BIGINT NOT NULL DEFAULT 0,

    stderr_tail TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (model_hash, experiment_id)
);

CREATE INDEX IF NOT EXISTS idx_model_artifacts_status
    ON model_artifacts(status);

CREATE INDEX IF NOT EXISTS idx_batches_model_hash
    ON experiment_batches(model_hash);

CREATE INDEX IF NOT EXISTS idx_batches_status
    ON experiment_batches(status);

CREATE INDEX IF NOT EXISTS idx_experiments_batch_id
    ON experiments(batch_id);

CREATE INDEX IF NOT EXISTS idx_experiments_status
    ON experiments(status);

CREATE INDEX IF NOT EXISTS idx_experiments_dispatcher_node_id
    ON experiments(dispatcher_node_id);