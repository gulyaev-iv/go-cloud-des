package main

import (
	"flag"
	"fmt"
	"time"
)

type Config struct {
	ListenAddr  string
	DatabaseURL string

	DefaultGOOS   string
	DefaultGOARCH string

	NodeTTL                   time.Duration
	NodeLostGracePeriod       time.Duration
	NodeLostReconcileInterval time.Duration
	MaxNodeFailures           int

	SchedulerInterval        time.Duration
	DispatcherRequestTimeout time.Duration

	ScaleOutAfter    time.Duration
	ScaleOutCooldown time.Duration

	NATSURL string

	CodegenStream         string
	CodegenSubject        string
	CodegenReplyPrefix    string
	CodegenRequestTimeout time.Duration

	EnableReflection bool
}

func parseConfig(args []string) (Config, error) {
	fs := flag.NewFlagSet("controlplane", flag.ContinueOnError)

	cfg := Config{}

	fs.StringVar(&cfg.ListenAddr, "listen-addr", ":8080", "gRPC listen address")
	fs.StringVar(&cfg.DatabaseURL, "database-url", "postgres://postgres:postgres@localhost:5432/go_cloud_des?sslmode=disable", "PostgreSQL connection URL")

	fs.StringVar(&cfg.DefaultGOOS, "default-goos", "linux", "default target GOOS for generated model binaries")
	fs.StringVar(&cfg.DefaultGOARCH, "default-goarch", "amd64", "default target GOARCH for generated model binaries")

	fs.DurationVar(&cfg.NodeTTL, "node-ttl", 30*time.Second, "dispatcher node TTL after last heartbeat")
	fs.DurationVar(&cfg.NodeLostGracePeriod, "node-lost-grace-period", 60*time.Second, "mark RUNNING/STARTING experiments as FAILED if their dispatcher is missing and they were not updated within this period")
	fs.DurationVar(&cfg.NodeLostReconcileInterval, "node-lost-reconcile-interval", 30*time.Second, "how often to scan for experiments whose dispatcher node is lost")
	fs.IntVar(&cfg.MaxNodeFailures, "max-node-failures", 3, "remove dispatcher node from registry after this many consecutive RPC failures")

	fs.DurationVar(&cfg.SchedulerInterval, "scheduler-interval", time.Second, "scheduler loop interval")
	fs.DurationVar(&cfg.DispatcherRequestTimeout, "dispatcher-request-timeout", 10*time.Second, "timeout for dispatcher gRPC requests")

	fs.DurationVar(&cfg.ScaleOutAfter, "scale-out-after", 5*time.Second, "request scale out if pending experiments wait longer than this")
	fs.DurationVar(&cfg.ScaleOutCooldown, "scale-out-cooldown", 30*time.Second, "minimum interval between scale out requests")

	fs.StringVar(&cfg.NATSURL, "nats-url", "nats://localhost:4222", "NATS server URL")
	fs.StringVar(&cfg.CodegenStream, "codegen-stream", "CODEGEN_TASKS", "JetStream stream name for codegen tasks")
	fs.StringVar(&cfg.CodegenSubject, "codegen-subject", "codegen.tasks", "NATS subject for codegen tasks")
	fs.StringVar(&cfg.CodegenReplyPrefix, "codegen-reply-prefix", "controlplane.codegen.reply", "NATS reply subject prefix for codegen responses")
	fs.DurationVar(&cfg.CodegenRequestTimeout, "codegen-request-timeout", 180*time.Second, "timeout for waiting codegen task result")

	fs.BoolVar(&cfg.EnableReflection, "enable-reflection", true, "enable gRPC reflection")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}

	if cfg.ListenAddr == "" {
		return cfg, fmt.Errorf("empty listen-addr")
	}
	if cfg.DatabaseURL == "" {
		return cfg, fmt.Errorf("empty database-url")
	}
	if cfg.DefaultGOOS == "" {
		return cfg, fmt.Errorf("empty default-goos")
	}
	if cfg.DefaultGOARCH == "" {
		return cfg, fmt.Errorf("empty default-goarch")
	}
	if cfg.NodeTTL <= 0 {
		return cfg, fmt.Errorf("node-ttl must be > 0")
	}
	if cfg.NodeLostGracePeriod <= 0 {
		return cfg, fmt.Errorf("node-lost-grace-period must be > 0")
	}
	if cfg.NodeLostReconcileInterval <= 0 {
		return cfg, fmt.Errorf("node-lost-reconcile-interval must be > 0")
	}
	if cfg.SchedulerInterval <= 0 {
		return cfg, fmt.Errorf("scheduler-interval must be > 0")
	}
	if cfg.DispatcherRequestTimeout <= 0 {
		return cfg, fmt.Errorf("dispatcher-request-timeout must be > 0")
	}
	if cfg.ScaleOutAfter <= 0 {
		return cfg, fmt.Errorf("scale-out-after must be > 0")
	}
	if cfg.ScaleOutCooldown <= 0 {
		return cfg, fmt.Errorf("scale-out-cooldown must be > 0")
	}
	if cfg.NATSURL == "" {
		return cfg, fmt.Errorf("empty nats-url")
	}
	if cfg.CodegenStream == "" {
		return cfg, fmt.Errorf("empty codegen-stream")
	}
	if cfg.CodegenSubject == "" {
		return cfg, fmt.Errorf("empty codegen-subject")
	}
	if cfg.CodegenReplyPrefix == "" {
		return cfg, fmt.Errorf("empty codegen-reply-prefix")
	}
	if cfg.CodegenRequestTimeout <= 0 {
		return cfg, fmt.Errorf("codegen-request-timeout must be > 0")
	}
	if cfg.MaxNodeFailures <= 0 {
		return cfg, fmt.Errorf("max-node-failures must be > 0")
	}

	return cfg, nil
}
