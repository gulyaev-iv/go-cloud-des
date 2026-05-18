package main

import (
	"context"
)

type NodeSnapshot struct {
	NodeID  string
	Address string

	RuntimeGOOS   string
	RuntimeGOARCH string

	TotalSlots uint64
	UsedSlots  uint64

	TotalMemoryBytes    uint64
	ReservedMemoryBytes uint64

	ActiveExperiments uint64
	CachedModels      uint64
}

type ControlPlaneClient interface {
	RegisterNode(ctx context.Context, snapshot NodeSnapshot) error
	SendHeartbeat(ctx context.Context, snapshot NodeSnapshot) error
	ReportExperimentResult(ctx context.Context, result ExperimentResult) error
}

type NoopControlPlaneClient struct{}

func (NoopControlPlaneClient) RegisterNode(ctx context.Context, snapshot NodeSnapshot) error {
	return nil
}

func (NoopControlPlaneClient) SendHeartbeat(ctx context.Context, snapshot NodeSnapshot) error {
	return nil
}

func (NoopControlPlaneClient) ReportExperimentResult(ctx context.Context, result ExperimentResult) error {
	return nil
}
