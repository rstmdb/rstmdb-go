package rstmdb

import "context"

// SnapshotInstance creates a point-in-time snapshot of an instance.
func (c *Client) SnapshotInstance(ctx context.Context, instanceID string) (*SnapshotResult, error) {
	params := map[string]any{
		"instance_id": instanceID,
	}
	var result SnapshotResult
	if err := c.doRequest(ctx, opSnapshotInstance, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
