package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	jsapi "github.com/nats-io/nats.go/jetstream"
)

const (
	codegenModeBuild = "build"
)

type CodegenClient struct {
	nc *nats.Conn
	js jsapi.JetStream

	streamName  string
	subject     string
	replyPrefix string
	timeout     time.Duration
}

type CodegenTask struct {
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

type CodegenTaskResult struct {
	RequestID string `json:"request_id"`
	OK        bool   `json:"ok"`

	ModelHash string `json:"model_hash,omitempty"`
	Mode      string `json:"mode,omitempty"`
	GOOS      string `json:"goos,omitempty"`
	GOARCH    string `json:"goarch,omitempty"`

	Source   *CodegenArtifactRef `json:"source,omitempty"`
	Binary   *CodegenArtifactRef `json:"binary,omitempty"`
	BuildLog *CodegenArtifactRef `json:"build_log,omitempty"`

	Stage     string `json:"stage,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
	Message   string `json:"message,omitempty"`
}

type CodegenArtifactRef struct {
	URI         string `json:"uri"`
	Key         string `json:"key"`
	SHA256      string `json:"sha256"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}

func NewCodegenClient(ctx context.Context, cfg Config) (*CodegenClient, error) {
	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		return nil, err
	}

	js, err := jsapi.New(nc)
	if err != nil {
		nc.Close()
		return nil, err
	}

	client := &CodegenClient{
		nc:          nc,
		js:          js,
		streamName:  cfg.CodegenStream,
		subject:     cfg.CodegenSubject,
		replyPrefix: strings.TrimSuffix(cfg.CodegenReplyPrefix, "."),
		timeout:     cfg.CodegenRequestTimeout,
	}

	if err := client.prepareStream(ctx); err != nil {
		nc.Close()
		return nil, err
	}

	return client, nil
}

func (c *CodegenClient) prepareStream(ctx context.Context) error {
	_, err := c.js.CreateOrUpdateStream(ctx, jsapi.StreamConfig{
		Name:      c.streamName,
		Subjects:  []string{c.subject},
		Retention: jsapi.WorkQueuePolicy,
	})

	if err != nil {
		return fmt.Errorf("prepare codegen stream: %w", err)
	}

	log.Printf(
		"codegen stream ready: stream=%s subject=%s retention=workqueue",
		c.streamName,
		c.subject,
	)

	return nil
}

func (c *CodegenClient) Close() {
	if c == nil || c.nc == nil {
		return
	}

	c.nc.Drain()
	c.nc.Close()
}

func (c *CodegenClient) BuildModel(ctx context.Context, task CodegenTask) (*CodegenTaskResult, error) {
	if c == nil {
		return nil, fmt.Errorf("empty codegen client")
	}
	if strings.TrimSpace(task.RequestID) == "" {
		return nil, fmt.Errorf("empty codegen request_id")
	}
	if strings.TrimSpace(task.ModelHash) == "" {
		return nil, fmt.Errorf("empty model_hash")
	}
	if strings.TrimSpace(task.DSL) == "" {
		return nil, fmt.Errorf("empty dsl")
	}
	if strings.TrimSpace(task.GOOS) == "" {
		return nil, fmt.Errorf("empty goos")
	}
	if strings.TrimSpace(task.GOARCH) == "" {
		return nil, fmt.Errorf("empty goarch")
	}

	if task.Mode == "" {
		task.Mode = codegenModeBuild
	}

	if task.ReplySubject == "" {
		task.ReplySubject = c.replyPrefix + "." + task.RequestID
	}

	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	replyCh := make(chan *nats.Msg, 1)

	sub, err := c.nc.ChanSubscribe(task.ReplySubject, replyCh)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil {
			log.Printf("codegen reply unsubscribe error: %v", err)
		}
	}()

	if err := c.nc.Flush(); err != nil {
		return nil, err
	}

	payload, err := json.Marshal(task)
	if err != nil {
		return nil, err
	}

	log.Printf(
		"codegen build requested: request_id=%s model_hash=%s goos=%s goarch=%s stream=%s subject=%s reply_subject=%s",
		task.RequestID,
		task.ModelHash,
		task.GOOS,
		task.GOARCH,
		c.streamName,
		c.subject,
		task.ReplySubject,
	)

	if _, err := c.js.Publish(callCtx, c.subject, payload); err != nil {
		return nil, err
	}

	select {
	case <-callCtx.Done():
		return nil, callCtx.Err()

	case msg := <-replyCh:
		var result CodegenTaskResult
		if err := json.Unmarshal(msg.Data, &result); err != nil {
			return nil, err
		}

		log.Printf(
			"codegen build result received: request_id=%s ok=%t model_hash=%s stage=%s error_code=%s",
			result.RequestID,
			result.OK,
			result.ModelHash,
			result.Stage,
			result.ErrorCode,
		)

		return &result, nil
	}
}
