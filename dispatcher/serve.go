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
	NodeID     string
	ListenAddr string

	ControlPlaneAddr    string
	ControlPlaneTimeout time.Duration

	Slots       uint64
	MemoryBytes uint64

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
	defer func() {
		if err := controlPlane.Close(); err != nil {
			log.Printf("controlplane client close error: %v", err)
		}
	}()

	server := NewDispatcherServer(cfg, store, cache, registry, controlPlane)
	server.StartWorkers(ctx)

	grpcServer := grpc.NewServer()
	pb.RegisterDispatcherServiceServer(grpcServer, server)
	reflection.Register(grpcServer)

	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return err
	}

	go heartbeatLoop(ctx, cfg, registry, cache, controlPlane)

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	log.Printf(
		"dispatcher started: node_id=%s listen=%s goos=%s goarch=%s slots=%d memory=%d",
		cfg.NodeID,
		cfg.ListenAddr,
		runtime.GOOS,
		runtime.GOARCH,
		cfg.Slots,
		cfg.MemoryBytes,
	)

	serveErr := grpcServer.Serve(listener)

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

	fs.StringVar(&cfg.ControlPlaneAddr, "control-plane-addr", "", "control plane gRPC address")
	fs.DurationVar(&cfg.ControlPlaneTimeout, "control-plane-timeout", 5*time.Second, "control plane gRPC request timeout")

	fs.Uint64Var(&cfg.Slots, "slots", 1, "execution slots")
	fs.Uint64Var(&cfg.MemoryBytes, "memory-bytes", 0, "total memory available for experiments")

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

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}

	if cfg.NodeID == "" {
		return cfg, fmt.Errorf("empty node-id")
	}
	if cfg.ListenAddr == "" {
		return cfg, fmt.Errorf("empty listen-addr")
	}
	if cfg.ControlPlaneAddr == "" {
		return cfg, fmt.Errorf("empty control-plane-addr")
	}
	if cfg.ControlPlaneTimeout <= 0 {
		return cfg, fmt.Errorf("control-plane-timeout must be > 0")
	}
	if cfg.Slots == 0 {
		return cfg, fmt.Errorf("slots must be > 0")
	}
	if cfg.HeartbeatInterval <= 0 {
		return cfg, fmt.Errorf("heartbeat-interval must be > 0")
	}
	if cfg.WorkDir == "" {
		return cfg, fmt.Errorf("empty work-dir")
	}

	return cfg, nil
}

func heartbeatLoop(ctx context.Context, cfg ServeConfig, registry *Registry, cache *ModelCache, controlPlane *GRPCControlPlaneClient) {
	if err := controlPlane.RegisterNode(ctx, nodeStatus(cfg, registry, cache)); err != nil {
		log.Printf("controlplane register node failed: %v", err)
	}

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
		Address: cfg.ListenAddr,

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
