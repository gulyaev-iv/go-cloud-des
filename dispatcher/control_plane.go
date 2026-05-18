package main

import (
	"context"
	"fmt"
	"log"
	"time"

	cpb "github.com/gulyaev-iv/go-cloud-des/api/controlplane/v1"
	dpb "github.com/gulyaev-iv/go-cloud-des/api/dispatcher/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GRPCControlPlaneClient struct {
	nodeID  string
	timeout time.Duration

	conn   *grpc.ClientConn
	client cpb.ControlPlaneNodeServiceClient
}

func NewGRPCControlPlaneClient(address string, nodeID string, timeout time.Duration) (*GRPCControlPlaneClient, error) {
	if address == "" {
		return nil, fmt.Errorf("empty control plane address")
	}
	if nodeID == "" {
		return nil, fmt.Errorf("empty node id")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	conn, err := grpc.NewClient(
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return &GRPCControlPlaneClient{
		nodeID:  nodeID,
		timeout: timeout,
		conn:    conn,
		client:  cpb.NewControlPlaneNodeServiceClient(conn),
	}, nil
}

func (c *GRPCControlPlaneClient) Close() error {
	if c.conn == nil {
		return nil
	}

	return c.conn.Close()
}

func (c *GRPCControlPlaneClient) RegisterNode(ctx context.Context, node *dpb.NodeStatus) error {
	callCtx, cancel := c.callContext(ctx)
	defer cancel()

	resp, err := c.client.RegisterNode(callCtx, &cpb.RegisterNodeRequest{
		Node: node,
	})
	if err != nil {
		return err
	}
	if !resp.GetAccepted() {
		return fmt.Errorf("register node rejected: %s", resp.GetMessage())
	}

	log.Printf("controlplane node registered: node_id=%s", node.GetNodeId())

	return nil
}

func (c *GRPCControlPlaneClient) SendHeartbeat(ctx context.Context, node *dpb.NodeStatus) error {
	callCtx, cancel := c.callContext(ctx)
	defer cancel()

	resp, err := c.client.SendHeartbeat(callCtx, &cpb.NodeHeartbeat{
		Node:         node,
		SentAtUnixMs: time.Now().UnixMilli(),
	})
	if err != nil {
		return err
	}
	if !resp.GetOk() {
		return fmt.Errorf("heartbeat rejected: %s", resp.GetMessage())
	}

	return nil
}

func (c *GRPCControlPlaneClient) ReportExperimentResult(ctx context.Context, result ExperimentResult) error {
	callCtx, cancel := c.callContext(ctx)
	defer cancel()

	resp, err := c.client.ReportExperimentResult(callCtx, &cpb.ReportExperimentResultRequest{
		NodeId: c.nodeID,
		Result: experimentResultToPB(result),
	})
	if err != nil {
		return err
	}
	if !resp.GetOk() {
		return fmt.Errorf("report experiment result rejected: %s", resp.GetMessage())
	}

	log.Printf(
		"experiment result reported: node_id=%s model_hash=%s experiment_id=%s status=%s",
		c.nodeID,
		result.ModelHash,
		result.ExperimentID,
		result.Status,
	)

	return nil
}

func (c *GRPCControlPlaneClient) callContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithTimeout(ctx, c.timeout)
}
