package rstmdb

import (
	"context"
	"encoding/json"
	"fmt"
)

// Client is the main rstmdb client.
type Client struct {
	conn       *conn
	serverInfo *HelloResult
}

// Connect creates a new client, connects to the server, performs the HELLO
// handshake, and optionally authenticates (if Auth is set).
// Pass nil for opts to use defaults.
func Connect(ctx context.Context, addr string, opts *Options) (*Client, error) {
	if opts == nil {
		opts = &Options{}
	}

	c, err := dial(ctx, addr, opts)
	if err != nil {
		return nil, err
	}

	info, err := c.handshake(ctx, opts)
	if err != nil {
		_ = c.netConn.Close()
		return nil, err
	}

	// Start the read loop after handshake completes
	go c.readLoop()

	return &Client{conn: c, serverInfo: info}, nil
}

// Close sends BYE and closes the connection.
func (c *Client) Close() error {
	return c.conn.close()
}

// ServerInfo returns a copy of the HELLO result received during handshake.
func (c *Client) ServerInfo() *HelloResult {
	if c.serverInfo == nil {
		return nil
	}
	info := *c.serverInfo
	if c.serverInfo.Features != nil {
		info.Features = append([]string(nil), c.serverInfo.Features...)
	}
	return &info
}

// NegotiatedFeatures returns a copy of the feature list negotiated during HELLO.
func (c *Client) NegotiatedFeatures() []string {
	if c.serverInfo == nil || len(c.serverInfo.Features) == 0 {
		return nil
	}
	return append([]string(nil), c.serverInfo.Features...)
}

// doRequest is the central dispatch: sends a request and unmarshals the result.
func (c *Client) doRequest(ctx context.Context, op operation, params any, result any) error {
	resp, err := c.conn.sendAndWait(ctx, op, params)
	if err != nil {
		return err
	}

	if resp.Status == "error" && resp.Error != nil {
		return resp.Error
	}
	if resp.Status != "ok" {
		return fmt.Errorf("rstmdb: unexpected status %q", resp.Status)
	}

	if result != nil && resp.Result != nil {
		if err := json.Unmarshal(resp.Result, result); err != nil {
			return fmt.Errorf("rstmdb: unmarshal result: %w", err)
		}
	}

	return nil
}

// Ping checks server connectivity.
func (c *Client) Ping(ctx context.Context) error {
	return c.doRequest(ctx, opPing, nil, nil)
}

// Info returns server information and capabilities.
func (c *Client) Info(ctx context.Context) (*ServerInfo, error) {
	var result ServerInfo
	if err := c.doRequest(ctx, opInfo, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ReplicationStatus returns the server's replication role and status.
func (c *Client) ReplicationStatus(ctx context.Context) (*ReplicationStatusResult, error) {
	var result ReplicationStatusResult
	if err := c.doRequest(ctx, opReplicationStatus, map[string]any{}, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
