package rstmdb

import "context"

// CreateInstance creates a new state machine instance.
func (c *Client) CreateInstance(ctx context.Context, req CreateInstanceRequest) (*CreateInstanceResult, error) {
	var result CreateInstanceResult
	if err := c.doRequest(ctx, opCreateInstance, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetInstance retrieves an instance by ID.
func (c *Client) GetInstance(ctx context.Context, instanceID string) (*Instance, error) {
	params := map[string]any{
		"instance_id": instanceID,
	}
	var result Instance
	if err := c.doRequest(ctx, opGetInstance, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListInstances lists instances with optional filters.
func (c *Client) ListInstances(ctx context.Context, opts ...ListOption) (*InstanceList, error) {
	params := map[string]any{}
	for _, opt := range opts {
		opt(params)
	}
	var result InstanceList
	if err := c.doRequest(ctx, opListInstances, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteInstance soft-deletes an instance.
func (c *Client) DeleteInstance(ctx context.Context, instanceID string, opts ...DeleteOption) (*DeleteInstanceResult, error) {
	params := map[string]any{
		"instance_id": instanceID,
	}
	for _, opt := range opts {
		opt(params)
	}
	var result DeleteInstanceResult
	if err := c.doRequest(ctx, opDeleteInstance, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
