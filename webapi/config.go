package main

import (
	"flag"
	"fmt"
	"time"
)

type Config struct {
	ListenAddr string

	ControlPlaneAddr    string
	ControlPlaneTimeout time.Duration

	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
	S3UseSSL    bool
	S3Region    string

	MaxRequestBytes int64

	EnableCORS      bool
	CORSAllowOrigin string

	ShutdownTimeout time.Duration
}

func parseConfig(args []string) (Config, error) {
	fs := flag.NewFlagSet("webapi", flag.ContinueOnError)

	cfg := Config{}

	fs.StringVar(&cfg.ListenAddr, "listen-addr", ":8081", "HTTP listen address")

	fs.StringVar(&cfg.ControlPlaneAddr, "control-plane-addr", "localhost:8080", "control plane gRPC address")
	fs.DurationVar(&cfg.ControlPlaneTimeout, "control-plane-timeout", 10*time.Second, "control plane gRPC request timeout")

	fs.StringVar(&cfg.S3Endpoint, "s3-endpoint", "localhost:9000", "S3-compatible storage endpoint")
	fs.StringVar(&cfg.S3AccessKey, "s3-access-key", "minioadmin", "S3 access key")
	fs.StringVar(&cfg.S3SecretKey, "s3-secret-key", "minioadmin", "S3 secret key")
	fs.StringVar(&cfg.S3Bucket, "s3-bucket", "cloud-des-artifacts", "S3 bucket")
	fs.BoolVar(&cfg.S3UseSSL, "s3-use-ssl", false, "use HTTPS for S3")
	fs.StringVar(&cfg.S3Region, "s3-region", "ru-1", "S3 region")

	fs.Int64Var(&cfg.MaxRequestBytes, "max-request-bytes", 32*1024*1024, "max JSON request body size")

	fs.BoolVar(&cfg.EnableCORS, "enable-cors", true, "enable CORS headers")
	fs.StringVar(&cfg.CORSAllowOrigin, "cors-allow-origin", "*", "CORS Access-Control-Allow-Origin value")

	fs.DurationVar(&cfg.ShutdownTimeout, "shutdown-timeout", 10*time.Second, "HTTP server shutdown timeout")

	if err := fs.Parse(args); err != nil {
		return cfg, err
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
	if cfg.MaxRequestBytes <= 0 {
		return cfg, fmt.Errorf("max-request-bytes must be > 0")
	}
	if cfg.EnableCORS && cfg.CORSAllowOrigin == "" {
		return cfg, fmt.Errorf("empty cors-allow-origin")
	}
	if cfg.ShutdownTimeout <= 0 {
		return cfg, fmt.Errorf("shutdown-timeout must be > 0")
	}

	return cfg, nil
}
