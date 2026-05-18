package main

import (
	"context"
	"log"
	"time"
)

type ScaleOutRequest struct {
	PendingExperiments int
	NeedNodes          int

	GOOS   string
	GOARCH string

	MemoryLimitBytes uint64

	OldestPendingFor time.Duration
}

type NodeProvisioner interface {
	RequestScaleOut(ctx context.Context, req ScaleOutRequest) error
}

type LoggingNodeProvisioner struct{}

func NewLoggingNodeProvisioner() *LoggingNodeProvisioner {
	return &LoggingNodeProvisioner{}
}

func (p *LoggingNodeProvisioner) RequestScaleOut(ctx context.Context, req ScaleOutRequest) error {
	log.Printf(
		"scale out requested: pending=%d need_nodes=%d goos=%s goarch=%s memory_limit=%d oldest_pending_for=%s",
		req.PendingExperiments,
		req.NeedNodes,
		req.GOOS,
		req.GOARCH,
		req.MemoryLimitBytes,
		req.OldestPendingFor,
	)

	return nil
}
