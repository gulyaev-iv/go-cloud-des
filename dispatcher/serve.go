package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	pb "github.com/gulyaev-iv/go-cloud-des/api/dispatcher/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type ServeConfig struct {
	NodeID        string
	ListenAddr    string
	AdvertiseAddr string

	ControlPlaneAddr    string
	ControlPlaneTimeout time.Duration

	RegistrationAttempts      int
	RegistrationRetryInterval time.Duration

	Slots       uint64
	MemoryBytes uint64

	CgroupEnabled bool
	CgroupRoot    string
	CgroupParent  string

	HeartbeatInterval time.Duration

	WorkDir string

	S3Endpoint     string
	S3AccessKey    string
	S3SecretKey    string
	S3Bucket       string
	S3UseSSL       bool
	S3Region       string
	S3Prefix       string
	S3CreateBucket bool

	ReportJournalDir    string
	ReportRetryInterval time.Duration
	ReportSendTimeout   time.Duration
	DrainTimeout        time.Duration
}

func runServe(args []string) error {
	cfg, err := parseServeConfig(args)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := NewS3Store(ctx, S3StoreConfig{
		Endpoint:     cfg.S3Endpoint,
		AccessKey:    cfg.S3AccessKey,
		SecretKey:    cfg.S3SecretKey,
		Bucket:       cfg.S3Bucket,
		UseSSL:       cfg.S3UseSSL,
		Region:       cfg.S3Region,
		Prefix:       cfg.S3Prefix,
		CreateBucket: cfg.S3CreateBucket,
	})
	if err != nil {
		return err
	}

	if err := os.RemoveAll(cfg.WorkDir); err != nil {
		return fmt.Errorf("clear work dir on startup: %w", err)
	}

	registry := NewRegistry(cfg.Slots, cfg.MemoryBytes)
	cache := NewModelCache(cfg.WorkDir, store)

	controlPlane, err := NewGRPCControlPlaneClient(cfg.ControlPlaneAddr, cfg.NodeID, cfg.ControlPlaneTimeout)
	if err != nil {
		return fmt.Errorf("connect controlplane: %w", err)
	}

	if err := registerNodeWithRetry(ctx, cfg, registry, cache, controlPlane); err != nil {
		if closeErr := controlPlane.Close(); closeErr != nil {
			log.Printf("controlplane client close error: %v", closeErr)
		}
		return err
	}

	reporter, err := NewReporter(cfg.ReportJournalDir, controlPlane, cfg.ReportRetryInterval, cfg.ReportSendTimeout)
	if err != nil {
		if closeErr := controlPlane.Close(); closeErr != nil {
			log.Printf("controlplane client close error: %v", closeErr)
		}
		return fmt.Errorf("init reporter: %w", err)
	}

	reporterCtx, reporterCancel := context.WithCancel(context.Background())
	reporter.Start(reporterCtx)

	server := NewDispatcherServer(cfg, store, cache, registry, controlPlane, reporter)
	server.StartWorkers(ctx)

	grpcServer := grpc.NewServer()
	pb.RegisterDispatcherServiceServer(grpcServer, server)
	reflection.Register(grpcServer)

	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		reporterCancel()
		reporter.Stop()
		if closeErr := controlPlane.Close(); closeErr != nil {
			log.Printf("controlplane client close error: %v", closeErr)
		}
		return err
	}

	go heartbeatLoop(ctx, cfg, registry, cache, controlPlane)

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	log.Printf(
		"dispatcher started: node_id=%s listen=%s advertise=%s goos=%s goarch=%s slots=%d memory=%d cgroup_enabled=%t cgroup_root=%s cgroup_parent=%s journal_dir=%s drain_timeout=%s",
		cfg.NodeID,
		cfg.ListenAddr,
		cfg.AdvertiseAddr,
		runtime.GOOS,
		runtime.GOARCH,
		cfg.Slots,
		cfg.MemoryBytes,
		cfg.CgroupEnabled,
		cfg.CgroupRoot,
		cfg.CgroupParent,
		cfg.ReportJournalDir,
		cfg.DrainTimeout,
	)

	serveErr := grpcServer.Serve(listener)

	drainCtx, drainCancel := context.WithTimeout(context.Background(), cfg.DrainTimeout)

	workersDone := make(chan struct{})
	go func() {
		server.WaitWorkers()
		close(workersDone)
	}()

	select {
	case <-workersDone:
		log.Printf("workers drained gracefully")
	case <-drainCtx.Done():
		cancelled := registry.CancelAll("dispatcher_shutdown")
		log.Printf("drain timeout exceeded, force-cancelled active experiments: count=%d", cancelled)
		server.WaitWorkers()
		log.Printf("workers stopped after force-cancel")
	}

	drainCancel()

	reporterCancel()
	reporter.Stop()

	if err := controlPlane.Close(); err != nil {
		log.Printf("controlplane client close error: %v", err)
	}

	if cleanupErr := cache.ClearWorkDir(); cleanupErr != nil {
		log.Printf("cleanup error: %v", cleanupErr)
	}

	if serveErr != nil {
		return serveErr
	}

	log.Printf("dispatcher stopped: node_id=%s", cfg.NodeID)

	return nil
}

func parseServeConfig(args []string) (ServeConfig, error) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)

	cfg := ServeConfig{}

	fs.StringVar(&cfg.NodeID, "node-id", "", "dispatcher node id")
	fs.StringVar(&cfg.ListenAddr, "listen-addr", ":9090", "gRPC listen address")
	fs.StringVar(&cfg.AdvertiseAddr, "advertise-addr", "", "dispatcher address advertised to control plane")

	fs.StringVar(&cfg.ControlPlaneAddr, "control-plane-addr", "", "control plane gRPC address")
	fs.DurationVar(&cfg.ControlPlaneTimeout, "control-plane-timeout", 5*time.Second, "control plane gRPC request timeout")

	fs.IntVar(&cfg.RegistrationAttempts, "registration-attempts", 12, "max attempts to register dispatcher node in control plane before startup fails")
	fs.DurationVar(&cfg.RegistrationRetryInterval, "registration-retry-interval", 5*time.Second, "interval between dispatcher registration attempts")

	fs.Uint64Var(&cfg.Slots, "slots", 1, "execution slots")
	fs.Uint64Var(&cfg.MemoryBytes, "memory-bytes", 0, "total memory available for experiments")

	fs.BoolVar(&cfg.CgroupEnabled, "cgroup-enabled", false, "enable cgroups v2 memory limits for model processes")
	fs.StringVar(&cfg.CgroupRoot, "cgroup-root", "/sys/fs/cgroup", "cgroups v2 mount root")
	fs.StringVar(&cfg.CgroupParent, "cgroup-parent", "go-cloud-des", "parent cgroup name for dispatcher experiments")

	fs.DurationVar(&cfg.HeartbeatInterval, "heartbeat-interval", 5*time.Second, "heartbeat interval")

	fs.StringVar(&cfg.WorkDir, "work-dir", "./dispatcher-work", "dispatcher working directory")

	fs.StringVar(&cfg.S3Endpoint, "s3-endpoint", "localhost:9000", "S3-compatible storage endpoint")
	fs.StringVar(&cfg.S3AccessKey, "s3-access-key", "minioadmin", "S3 access key")
	fs.StringVar(&cfg.S3SecretKey, "s3-secret-key", "minioadmin", "S3 secret key")
	fs.StringVar(&cfg.S3Bucket, "s3-bucket", "cloud-des-artifacts", "S3 bucket")
	fs.BoolVar(&cfg.S3UseSSL, "s3-use-ssl", false, "use HTTPS for S3")
	fs.StringVar(&cfg.S3Region, "s3-region", "ru-1", "S3 region")
	fs.StringVar(&cfg.S3Prefix, "s3-prefix", "", "S3 prefix for dispatcher-created objects")
	fs.BoolVar(&cfg.S3CreateBucket, "s3-create-bucket", true, "create S3 bucket if it does not exist")

	fs.StringVar(&cfg.ReportJournalDir, "report-journal-dir", "./dispatcher-reports", "directory for persistent experiment result journal")
	fs.DurationVar(&cfg.ReportRetryInterval, "report-retry-interval", 10*time.Second, "interval for retrying unreported experiment results")
	fs.DurationVar(&cfg.ReportSendTimeout, "report-send-timeout", 5*time.Second, "timeout for a single report attempt")
	fs.DurationVar(&cfg.DrainTimeout, "drain-timeout", 60*time.Second, "max time to wait for active experiments to finish on shutdown before force-cancelling")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}

	if cfg.NodeID == "" {
		return cfg, fmt.Errorf("empty node-id")
	}
	if cfg.ListenAddr == "" {
		return cfg, fmt.Errorf("empty listen-addr")
	}
	if cfg.AdvertiseAddr == "" {
		return cfg, fmt.Errorf("empty advertise-addr")
	}
	if cfg.ControlPlaneAddr == "" {
		return cfg, fmt.Errorf("empty control-plane-addr")
	}
	if cfg.ControlPlaneTimeout <= 0 {
		return cfg, fmt.Errorf("control-plane-timeout must be > 0")
	}
	if cfg.RegistrationAttempts <= 0 {
		return cfg, fmt.Errorf("registration-attempts must be > 0")
	}
	if cfg.RegistrationRetryInterval <= 0 {
		return cfg, fmt.Errorf("registration-retry-interval must be > 0")
	}
	if cfg.Slots == 0 {
		return cfg, fmt.Errorf("slots must be > 0")
	}
	if cfg.HeartbeatInterval <= 0 {
		return cfg, fmt.Errorf("heartbeat-interval must be > 0")
	}
	if cfg.CgroupEnabled {
		if cfg.CgroupRoot == "" {
			return cfg, fmt.Errorf("empty cgroup-root")
		}
		if cfg.CgroupParent == "" {
			return cfg, fmt.Errorf("empty cgroup-parent")
		}
	}
	if cfg.WorkDir == "" {
		return cfg, fmt.Errorf("empty work-dir")
	}
	if cfg.ReportJournalDir == "" {
		return cfg, fmt.Errorf("empty report-journal-dir")
	}
	if cfg.ReportRetryInterval <= 0 {
		return cfg, fmt.Errorf("report-retry-interval must be > 0")
	}
	if cfg.ReportSendTimeout <= 0 {
		return cfg, fmt.Errorf("report-send-timeout must be > 0")
	}
	if cfg.DrainTimeout <= 0 {
		return cfg, fmt.Errorf("drain-timeout must be > 0")
	}

	return cfg, nil
}

func registerNodeWithRetry(ctx context.Context, cfg ServeConfig, registry *Registry, cache *ModelCache, controlPlane *GRPCControlPlaneClient) error {
	var lastErr error

	for attempt := 1; attempt <= cfg.RegistrationAttempts; attempt++ {
		if err := controlPlane.RegisterNode(ctx, nodeStatus(cfg, registry, cache)); err != nil {
			lastErr = err

			log.Printf(
				"controlplane register node failed: node_id=%s attempt=%d/%d retry_in=%s error=%v",
				cfg.NodeID,
				attempt,
				cfg.RegistrationAttempts,
				cfg.RegistrationRetryInterval,
				err,
			)

			if attempt == cfg.RegistrationAttempts {
				break
			}

			timer := time.NewTimer(cfg.RegistrationRetryInterval)

			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return ctx.Err()

			case <-timer.C:
			}

			continue
		}

		log.Printf(
			"controlplane node registration completed: node_id=%s attempts=%d",
			cfg.NodeID,
			attempt,
		)

		return nil
	}

	return fmt.Errorf(
		"register dispatcher node in control plane failed after %d attempts: %w",
		cfg.RegistrationAttempts,
		lastErr,
	)
}

func heartbeatLoop(ctx context.Context, cfg ServeConfig, registry *Registry, cache *ModelCache, controlPlane *GRPCControlPlaneClient) {
	ticker := time.NewTicker(cfg.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			if err := controlPlane.SendHeartbeat(ctx, nodeStatus(cfg, registry, cache)); err != nil {
				log.Printf("controlplane heartbeat failed: %v", err)
			}
		}
	}
}

func nodeStatus(cfg ServeConfig, registry *Registry, cache *ModelCache) *pb.NodeStatus {
	snap := registry.Snapshot()

	return &pb.NodeStatus{
		NodeId:  cfg.NodeID,
		Address: cfg.AdvertiseAddr,

		RuntimeGoos:   runtime.GOOS,
		RuntimeGoarch: runtime.GOARCH,

		TotalSlots: snap.TotalSlots,
		UsedSlots:  snap.UsedSlots,

		TotalMemoryBytes:    snap.TotalMemoryBytes,
		ReservedMemoryBytes: snap.ReservedMemoryBytes,

		ActiveExperiments: snap.ActiveExperiments,
		CachedModels:      uint64(cache.Len()),
	}
}
