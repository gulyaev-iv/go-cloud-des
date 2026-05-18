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

	NodeTTL time.Duration

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

	return cfg, nil
}
