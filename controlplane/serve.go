package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	cpb "github.com/gulyaev-iv/go-cloud-des/api/controlplane/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func runServe(args []string) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := OpenDB(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := ApplySchema(ctx, db); err != nil {
		return err
	}

	repo := NewRepository(db)
	nodes := NewNodeRegistry(cfg.NodeTTL)

	codegen, err := NewCodegenClient(ctx, cfg)
	if err != nil {
		return err
	}
	defer codegen.Close()

	server := NewControlPlaneServer(cfg, repo, nodes, codegen)

	grpcServer := grpc.NewServer()

	cpb.RegisterControlPlaneServiceServer(grpcServer, server)
	cpb.RegisterControlPlaneNodeServiceServer(grpcServer, server)

	if cfg.EnableReflection {
		reflection.Register(grpcServer)
	}

	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return err
	}

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	log.Printf(
		"controlplane started: listen=%s default_goos=%s default_goarch=%s node_ttl=%s nats_url=%s codegen_stream=%s codegen_subject=%s",
		cfg.ListenAddr,
		cfg.DefaultGOOS,
		cfg.DefaultGOARCH,
		cfg.NodeTTL,
		cfg.NATSURL,
		cfg.CodegenStream,
		cfg.CodegenSubject,
	)

	serveErr := grpcServer.Serve(listener)
	if serveErr != nil {
		return serveErr
	}

	log.Printf("controlplane stopped")

	return nil
}
