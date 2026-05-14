package main

const (
	taskModeGenerate = "generate"
	taskModeBuild    = "build"

	stageDecode          = "decode"
	stageValidateRequest = "validate_request"
	stageGenerate        = "generate"
	stageStoreSource     = "store_source"
	stageBuild           = "build"
	stageStoreBinary     = "store_binary"
	stageStoreBuildLog   = "store_build_log"

	errorDecode        = "decode_error"
	errorBadRequest    = "bad_request"
	errorGenerate      = "generation_error"
	errorStoreSource   = "store_source_error"
	errorBuild         = "build_error"
	errorStoreBinary   = "store_binary_error"
	errorStoreBuildLog = "store_build_log_error"
)

type Task struct {
	RequestID    string `json:"request_id"`
	ReplySubject string `json:"reply_subject,omitempty"`

	ModelHash string `json:"model_hash"`
	DSL       string `json:"dsl"`

	Mode   string `json:"mode"`
	GOOS   string `json:"goos,omitempty"`
	GOARCH string `json:"goarch,omitempty"`

	StoreSource *bool `json:"store_source,omitempty"`
	StoreBinary *bool `json:"store_binary,omitempty"`

	BuildTimeoutSec int `json:"build_timeout_sec,omitempty"`
}

type TaskResult struct {
	RequestID string `json:"request_id"`
	OK        bool   `json:"ok"`

	ModelHash string `json:"model_hash,omitempty"`
	Mode      string `json:"mode,omitempty"`
	GOOS      string `json:"goos,omitempty"`
	GOARCH    string `json:"goarch,omitempty"`

	Source   *ArtifactRef `json:"source,omitempty"`
	Binary   *ArtifactRef `json:"binary,omitempty"`
	BuildLog *ArtifactRef `json:"build_log,omitempty"`

	Stage     string `json:"stage,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
	Message   string `json:"message,omitempty"`
}

type ArtifactRef struct {
	URI         string `json:"uri"`
	Key         string `json:"key"`
	SHA256      string `json:"sha256"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}
