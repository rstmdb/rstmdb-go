package rstmdb

import (
	"context"
	"encoding/json"
	"fmt"
)

func (c *conn) handshake(ctx context.Context, opts *Options) (*HelloResult, error) {
	// Build HELLO params
	params := map[string]any{
		"protocol_version": 1,
	}
	if opts.ClientName != "" {
		params["client_name"] = opts.ClientName
	}
	wireMode := opts.WireMode
	if wireMode == "" {
		wireMode = "binary_json"
	}
	params["wire_mode"] = wireMode
	features := opts.Features
	if len(features) == 0 {
		features = []string{"idempotency", "durable_idempotency", "batch"}
	}
	params["features"] = features

	id := c.nextRequestID()
	req := &request{
		Type:   "request",
		ID:     id,
		Op:     string(opHello),
		Params: params,
	}

	// HELLO always uses binary framing (before wire mode negotiation)
	if err := c.sendRequest(req); err != nil {
		return nil, fmt.Errorf("rstmdb: HELLO send: %w", err)
	}

	// Read response directly (readLoop not started yet)
	payload, err := decodeFrame(c.reader)
	if err != nil {
		return nil, fmt.Errorf("rstmdb: HELLO read: %w", err)
	}

	var resp rawResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, fmt.Errorf("rstmdb: HELLO parse: %w", err)
	}

	if resp.Status == "error" && resp.Error != nil {
		return nil, resp.Error
	}
	if resp.Status != "ok" {
		return nil, fmt.Errorf("rstmdb: HELLO failed with status %q", resp.Status)
	}

	var result HelloResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("rstmdb: HELLO result parse: %w", err)
	}

	// Switch wire mode if server negotiated JSONL
	if result.WireMode == "jsonl" {
		c.wireMode = "jsonl"
	}

	// Authenticate if token provided
	if opts.Auth != "" {
		if err := c.authenticate(ctx, opts.Auth); err != nil {
			return nil, err
		}
	}

	return &result, nil
}

func (c *conn) authenticate(ctx context.Context, token string) error {
	params := map[string]any{
		"method": "bearer",
		"token":  token,
	}

	id := c.nextRequestID()
	req := &request{
		Type:   "request",
		ID:     id,
		Op:     string(opAuth),
		Params: params,
	}

	if err := c.sendRequest(req); err != nil {
		return fmt.Errorf("rstmdb: AUTH send: %w", err)
	}

	// Still reading directly (before readLoop)
	var payload []byte
	var err error
	if c.wireMode == "jsonl" {
		payload, err = c.reader.ReadBytes('\n')
	} else {
		payload, err = decodeFrame(c.reader)
	}
	if err != nil {
		return fmt.Errorf("rstmdb: AUTH read: %w", err)
	}

	var resp rawResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return fmt.Errorf("rstmdb: AUTH parse: %w", err)
	}

	if resp.Status == "error" && resp.Error != nil {
		return resp.Error
	}
	if resp.Status != "ok" {
		return fmt.Errorf("rstmdb: AUTH failed with status %q", resp.Status)
	}

	return nil
}
