package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	jsapi "github.com/nats-io/nats.go/jetstream"
)

type ServeConfig struct {
	Workers int

	NATSURL          string
	NATSStream       string
	NATSSubject      string
	NATSConsumer     string
	NATSCreateStream bool
	NATSFetchWait    time.Duration
	NATSAckWait      time.Duration
	NATSPublishWait  time.Duration

	ArtifactStore string
	ArtifactDir   string

	S3Endpoint     string
	S3AccessKey    string
	S3SecretKey    string
	S3Bucket       string
	S3UseSSL       bool
	S3Region       string
	S3Prefix       string
	S3CreateBucket bool

	ModuleDir     string
	WorkDirRoot   string
	DefaultGOOS   string
	DefaultGOARCH string
	BuildTimeout  time.Duration
	KeepWorkDir   bool
}

func runServe(args []string) error {
	cfg, err := parseServeConfig(args)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := newServeStore(ctx, cfg)
	if err != nil {
		return err
	}

	nc, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		return err
	}
	defer nc.Close()

	js, err := jsapi.New(nc)
	if err != nil {
		return err
	}

	consumer, err := prepareJetStreamConsumer(ctx, js, cfg)
	if err != nil {
		return err
	}

	processCfg := ProcessConfig{
		ModuleDir:     cfg.ModuleDir,
		WorkDirRoot:   cfg.WorkDirRoot,
		DefaultGOOS:   cfg.DefaultGOOS,
		DefaultGOARCH: cfg.DefaultGOARCH,
		BuildTimeout:  cfg.BuildTimeout,
		KeepWorkDir:   cfg.KeepWorkDir,
	}

	tasks := make(chan jsapi.Msg)
	available := make(chan struct{}, cfg.Workers)

	var wg sync.WaitGroup

	for i := 0; i < cfg.Workers; i++ {
		available <- struct{}{}

		wg.Add(1)
		go serveWorker(&wg, i, cfg, processCfg, store, nc, tasks, available)
	}

	log.Printf(
		"codegen server started: workers=%d nats=%s stream=%s subject=%s consumer=%s artifact_store=%s",
		cfg.Workers,
		cfg.NATSURL,
		cfg.NATSStream,
		cfg.NATSSubject,
		cfg.NATSConsumer,
		artifactStoreDescription(cfg),
	)

	err = fetchLoop(ctx, cfg, consumer, tasks, available)

	close(tasks)
	wg.Wait()

	if err != nil {
		return err
	}

	log.Printf("codegen server stopped")
	return nil
}

func newServeStore(ctx context.Context, cfg ServeConfig) (Store, error) {
	switch cfg.ArtifactStore {
	case "fs":
		return NewFSStore(cfg.ArtifactDir)

	case "s3":
		return NewS3Store(ctx, S3StoreConfig{
			Endpoint:     cfg.S3Endpoint,
			AccessKey:    cfg.S3AccessKey,
			SecretKey:    cfg.S3SecretKey,
			Bucket:       cfg.S3Bucket,
			UseSSL:       cfg.S3UseSSL,
			Region:       cfg.S3Region,
			Prefix:       cfg.S3Prefix,
			CreateBucket: cfg.S3CreateBucket,
		})

	default:
		return nil, fmt.Errorf("unknown artifact store %q", cfg.ArtifactStore)
	}
}

func artifactStoreDescription(cfg ServeConfig) string {
	switch cfg.ArtifactStore {
	case "fs":
		return fmt.Sprintf("fs:%s", cfg.ArtifactDir)

	case "s3":
		if cfg.S3Prefix == "" {
			return fmt.Sprintf("s3://%s endpoint=%s", cfg.S3Bucket, cfg.S3Endpoint)
		}

		return fmt.Sprintf("s3://%s/%s endpoint=%s", cfg.S3Bucket, cfg.S3Prefix, cfg.S3Endpoint)

	default:
		return cfg.ArtifactStore
	}
}

func parseServeConfig(args []string) (ServeConfig, error) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)

	cfg := ServeConfig{}

	fs.IntVar(&cfg.Workers, "workers", runtime.NumCPU(), "number of codegen worker goroutines")

	fs.StringVar(&cfg.NATSURL, "nats-url", nats.DefaultURL, "NATS server URL")
	fs.StringVar(&cfg.NATSStream, "nats-stream", "CODEGEN_TASKS", "JetStream stream name")
	fs.StringVar(&cfg.NATSSubject, "nats-subject", "codegen.tasks", "JetStream task subject")
	fs.StringVar(&cfg.NATSConsumer, "nats-consumer", "codegen-worker", "JetStream durable consumer name")
	fs.BoolVar(&cfg.NATSCreateStream, "nats-create-stream", true, "create or update JetStream stream if needed")
	fs.DurationVar(&cfg.NATSFetchWait, "nats-fetch-wait", time.Second, "max wait for one JetStream fetch")
	fs.DurationVar(&cfg.NATSAckWait, "nats-ack-wait", 10*time.Minute, "JetStream ack wait for build tasks")
	fs.DurationVar(&cfg.NATSPublishWait, "nats-publish-wait", 5*time.Second, "max wait for publishing result")

	fs.StringVar(&cfg.ArtifactStore, "artifact-store", "fs", "artifact store backend: fs or s3")
	fs.StringVar(&cfg.ArtifactDir, "artifact-dir", "./artifacts", "filesystem artifact directory")

	fs.StringVar(&cfg.S3Endpoint, "s3-endpoint", "localhost:9000", "S3-compatible storage endpoint")
	fs.StringVar(&cfg.S3AccessKey, "s3-access-key", "minioadmin", "S3 access key")
	fs.StringVar(&cfg.S3SecretKey, "s3-secret-key", "minioadmin", "S3 secret key")
	fs.StringVar(&cfg.S3Bucket, "s3-bucket", "cloud-des-artifacts", "S3 bucket for artifacts")
	fs.BoolVar(&cfg.S3UseSSL, "s3-use-ssl", false, "use HTTPS for S3-compatible storage")
	fs.StringVar(&cfg.S3Region, "s3-region", "us-east-1", "S3 region")
	fs.StringVar(&cfg.S3Prefix, "s3-prefix", "", "S3 object key prefix")
	fs.BoolVar(&cfg.S3CreateBucket, "s3-create-bucket", true, "create S3 bucket if it does not exist")

	fs.StringVar(&cfg.ModuleDir, "module-dir", ".", "local path to codegen module")
	fs.StringVar(&cfg.WorkDirRoot, "work-dir", "", "temporary build work directory root")
	fs.StringVar(&cfg.DefaultGOOS, "default-goos", runtime.GOOS, "default target GOOS")
	fs.StringVar(&cfg.DefaultGOARCH, "default-goarch", runtime.GOARCH, "default target GOARCH")
	fs.DurationVar(&cfg.BuildTimeout, "build-timeout", 120*time.Second, "default go build timeout")
	fs.BoolVar(&cfg.KeepWorkDir, "keep-workdir", false, "do not delete temporary build directories")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}

	if cfg.Workers <= 0 {
		return cfg, fmt.Errorf("workers must be > 0")
	}
	if cfg.NATSURL == "" {
		return cfg, fmt.Errorf("empty nats-url")
	}
	if cfg.NATSStream == "" {
		return cfg, fmt.Errorf("empty nats-stream")
	}
	if cfg.NATSSubject == "" {
		return cfg, fmt.Errorf("empty nats-subject")
	}
	if cfg.NATSConsumer == "" {
		return cfg, fmt.Errorf("empty nats-consumer")
	}
	if cfg.NATSFetchWait <= 0 {
		return cfg, fmt.Errorf("nats-fetch-wait must be > 0")
	}
	if cfg.NATSAckWait <= 0 {
		return cfg, fmt.Errorf("nats-ack-wait must be > 0")
	}
	if cfg.NATSPublishWait <= 0 {
		return cfg, fmt.Errorf("nats-publish-wait must be > 0")
	}

	switch cfg.ArtifactStore {
	case "fs":
		if cfg.ArtifactDir == "" {
			return cfg, fmt.Errorf("empty artifact-dir")
		}

	case "s3":
		if cfg.S3Endpoint == "" {
			return cfg, fmt.Errorf("empty s3-endpoint")
		}
		if cfg.S3AccessKey == "" {
			return cfg, fmt.Errorf("empty s3-access-key")
		}
		if cfg.S3SecretKey == "" {
			return cfg, fmt.Errorf("empty s3-secret-key")
		}
		if cfg.S3Bucket == "" {
			return cfg, fmt.Errorf("empty s3-bucket")
		}
		if cfg.S3Region == "" {
			return cfg, fmt.Errorf("empty s3-region")
		}

	default:
		return cfg, fmt.Errorf("unknown artifact-store %q, expected fs or s3", cfg.ArtifactStore)
	}

	if cfg.DefaultGOOS == "" {
		return cfg, fmt.Errorf("empty default-goos")
	}
	if cfg.DefaultGOARCH == "" {
		return cfg, fmt.Errorf("empty default-goarch")
	}
	if cfg.BuildTimeout <= 0 {
		return cfg, fmt.Errorf("build-timeout must be > 0")
	}

	return cfg, nil
}

func prepareJetStreamConsumer(ctx context.Context, js jsapi.JetStream, cfg ServeConfig) (jsapi.Consumer, error) {

	streamCfg := jsapi.StreamConfig{
		Name:      cfg.NATSStream,
		Subjects:  []string{cfg.NATSSubject},
		Retention: jsapi.WorkQueuePolicy,
	}

	var stream jsapi.Stream
	var err error

	if cfg.NATSCreateStream {
		stream, err = js.CreateOrUpdateStream(ctx, streamCfg)
	} else {
		stream, err = js.Stream(ctx, cfg.NATSStream)
	}
	if err != nil {
		return nil, err
	}

	return stream.CreateOrUpdateConsumer(ctx, jsapi.ConsumerConfig{
		Durable:         cfg.NATSConsumer,
		AckPolicy:       jsapi.AckExplicitPolicy,
		AckWait:         cfg.NATSAckWait,
		FilterSubject:   cfg.NATSSubject,
		MaxRequestBatch: 1,
	})
}

func fetchLoop(ctx context.Context, cfg ServeConfig, consumer jsapi.Consumer, tasks chan<- jsapi.Msg, available chan struct{}) error {
	for {
		select {
		case <-ctx.Done():
			return nil

		case <-available:
		}

		msg, err := fetchOne(cfg, consumer)
		if err != nil {
			available <- struct{}{}

			if errors.Is(err, jsapi.ErrNoMessages) {
				continue
			}

			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || ctx.Err() != nil {
				return nil
			}

			log.Printf("fetch error: %v", err)
			time.Sleep(time.Second)
			continue
		}

		if msg == nil {
			available <- struct{}{}
			continue
		}

		select {
		case tasks <- msg:

		case <-ctx.Done():
			available <- struct{}{}
			_ = msg.Nak()
			return nil
		}
	}
}

func fetchOne(cfg ServeConfig, consumer jsapi.Consumer) (jsapi.Msg, error) {
	batch, err := consumer.Fetch(1, jsapi.FetchMaxWait(cfg.NATSFetchWait))
	if err != nil {
		return nil, err
	}

	for msg := range batch.Messages() {
		return msg, nil
	}

	if err := batch.Error(); err != nil {
		return nil, err
	}

	return nil, nil
}

func serveWorker(wg *sync.WaitGroup, workerID int, cfg ServeConfig, processCfg ProcessConfig, store Store, nc *nats.Conn, tasks <-chan jsapi.Msg, available chan<- struct{}) {
	defer wg.Done()

	for msg := range tasks {
		handleTaskMessage(workerID, cfg, processCfg, store, nc, msg)

		available <- struct{}{}
	}
}

func handleTaskMessage(workerID int, cfg ServeConfig, processCfg ProcessConfig, store Store, nc *nats.Conn, msg jsapi.Msg) {
	var task Task

	if err := json.Unmarshal(msg.Data(), &task); err != nil {
		result := TaskResult{
			OK:        false,
			Stage:     stageDecode,
			ErrorCode: errorDecode,
			Message:   err.Error(),
		}

		replySubject := msg.Reply()
		if replySubject == "" {
			log.Printf("worker=%d decode error without reply subject: %v", workerID, err)
			if ackErr := msg.Ack(); ackErr != nil {
				log.Printf("worker=%d ack error after decode failure: %v", workerID, ackErr)
			}
			return
		}

		if err := publishTaskResult(nc, replySubject, result, cfg.NATSPublishWait); err != nil {
			log.Printf("worker=%d publish decode error failed: %v", workerID, err)
			_ = msg.Nak()
			return
		}

		if err := msg.Ack(); err != nil {
			log.Printf("worker=%d ack error after decode failure: %v", workerID, err)
		}
		return
	}

	replySubject := task.ReplySubject
	if replySubject == "" {
		replySubject = msg.Reply()
	}

	result := processTask(context.Background(), processCfg, store, task)

	if replySubject == "" {
		log.Printf("worker=%d request_id=%s no reply subject, result dropped ok=%v stage=%s error=%s",
			workerID, task.RequestID, result.OK, result.Stage, result.ErrorCode,
		)

		if err := msg.Ack(); err != nil {
			log.Printf("worker=%d ack error without reply subject: %v", workerID, err)
		}
		return
	}

	if err := publishTaskResult(nc, replySubject, result, cfg.NATSPublishWait); err != nil {
		log.Printf(
			"worker=%d request_id=%s publish result failed: %v",
			workerID, task.RequestID, err,
		)

		_ = msg.Nak()
		return
	}

	if err := msg.Ack(); err != nil {
		log.Printf("worker=%d request_id=%s ack error: %v", workerID, task.RequestID, err)
		return
	}

	log.Printf(
		"worker=%d request_id=%s processed ok=%v mode=%s hash=%s stage=%s error=%s",
		workerID, result.RequestID, result.OK, result.Mode, result.ModelHash, result.Stage, result.ErrorCode,
	)
}

func publishTaskResult(nc *nats.Conn, subject string, result TaskResult, wait time.Duration) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}

	if err := nc.Publish(subject, data); err != nil {
		return err
	}

	return nc.FlushTimeout(wait)
}
