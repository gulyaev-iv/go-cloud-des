package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	controlPlane, err := NewControlPlaneClient(cfg.ControlPlaneAddr, cfg.ControlPlaneTimeout)
	if err != nil {
		return fmt.Errorf("connect controlplane: %w", err)
	}
	defer func() {
		if err := controlPlane.Close(); err != nil {
			log.Printf("controlplane client close error: %v", err)
		}
	}()

	store, err := NewS3Store(ctx, S3StoreConfig{
		Endpoint:  cfg.S3Endpoint,
		AccessKey: cfg.S3AccessKey,
		SecretKey: cfg.S3SecretKey,
		Bucket:    cfg.S3Bucket,
		UseSSL:    cfg.S3UseSSL,
		Region:    cfg.S3Region,
	})
	if err != nil {
		return fmt.Errorf("connect s3: %w", err)
	}

	api := NewServer(cfg, controlPlane, store)

	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           api.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("webapi shutdown error: %v", err)
		}
	}()

	log.Printf(
		"webapi started: listen=%s control_plane=%s s3_endpoint=%s s3_bucket=%s cors=%t",
		cfg.ListenAddr,
		cfg.ControlPlaneAddr,
		cfg.S3Endpoint,
		cfg.S3Bucket,
		cfg.EnableCORS,
	)

	err = httpServer.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	log.Printf("webapi stopped")

	return nil
}
