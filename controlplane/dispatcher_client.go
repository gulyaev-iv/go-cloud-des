package main

import (
	"context"
	"fmt"
	"time"

	dpb "github.com/gulyaev-iv/go-cloud-des/api/dispatcher/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type DispatcherClient struct {
	address string
	timeout time.Duration

	conn   *grpc.ClientConn
	client dpb.DispatcherServiceClient
}

func NewDispatcherClient(address string, timeout time.Duration) (*DispatcherClient, error) {
	if address == "" {
		return nil, fmt.Errorf("empty dispatcher address")
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return &DispatcherClient{
		address: address,
		timeout: timeout,
		conn:    conn,
		client:  dpb.NewDispatcherServiceClient(conn),
	}, nil
}

func (c *DispatcherClient) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}

	return c.conn.Close()
}

func (c *DispatcherClient) StartExperiment(ctx context.Context, req *dpb.StartExperimentRequest) (*dpb.StartExperimentResponse, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.StartExperiment(callCtx, req)
}

func (c *DispatcherClient) StopExperiment(ctx context.Context, req *dpb.StopExperimentRequest) (*dpb.StopExperimentResponse, error) {
	callCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	return c.client.StopExperiment(callCtx, req)
}
