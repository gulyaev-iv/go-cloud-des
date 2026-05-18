package main

import (
	"context"
	"time"

	cpb "github.com/gulyaev-iv/go-cloud-des/api/controlplane/v1"
	dpb "github.com/gulyaev-iv/go-cloud-des/api/dispatcher/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ControlPlaneClient struct {
	conn    *grpc.ClientConn
	client  cpb.ControlPlaneServiceClient
	timeout time.Duration
}

func NewControlPlaneClient(addr string, timeout time.Duration) (*ControlPlaneClient, error) {
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return &ControlPlaneClient{
		conn:    conn,
		client:  cpb.NewControlPlaneServiceClient(conn),
		timeout: timeout,
	}, nil
}

func (c *ControlPlaneClient) Close() error {
	return c.conn.Close()
}

func (c *ControlPlaneClient) SubmitExperimentBatch(ctx context.Context, req *cpb.SubmitExperimentBatchRequest) (*cpb.SubmitExperimentBatchResponse, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.SubmitExperimentBatch(callCtx, req)
}

func (c *ControlPlaneClient) GetExperimentBatch(ctx context.Context, batchID string) (*cpb.ExperimentBatchStatus, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.GetExperimentBatch(callCtx, &cpb.GetExperimentBatchRequest{
		BatchId: batchID,
	})
}

func (c *ControlPlaneClient) GetExperiment(ctx context.Context, modelHash string, experimentID string) (*dpb.ExperimentStatus, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.GetExperiment(callCtx, &cpb.GetExperimentRequest{
		ModelHash:    modelHash,
		ExperimentId: experimentID,
	})
}

func (c *ControlPlaneClient) CancelExperiment(ctx context.Context, modelHash string, experimentID string) (*cpb.CancelExperimentResponse, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.CancelExperiment(callCtx, &cpb.CancelExperimentRequest{
		ModelHash:    modelHash,
		ExperimentId: experimentID,
	})
}

func (c *ControlPlaneClient) ListNodes(ctx context.Context, status string) (*cpb.ListNodesResponse, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.ListNodes(callCtx, &cpb.ListNodesRequest{
		Status: status,
	})
}
