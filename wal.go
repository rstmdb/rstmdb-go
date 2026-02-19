package rstmdb

import "context"

// WALRead reads entries from the WAL starting at the given offset.
func (c *Client) WALRead(ctx context.Context, fromOffset uint64, opts ...WALReadOption) (*WALReadResult, error) {
	params := map[string]any{
		"from_offset": fromOffset,
	}
	for _, opt := range opts {
		opt(params)
	}
	var result WALReadResult
	if err := c.doRequest(ctx, opWALRead, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// WALStats returns WAL statistics.
func (c *Client) WALStats(ctx context.Context) (*WALStatsResult, error) {
	var result WALStatsResult
	if err := c.doRequest(ctx, opWALStats, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Compact triggers WAL compaction.
func (c *Client) Compact(ctx context.Context, forceSnapshot bool) (*CompactResult, error) {
	params := map[string]any{
		"force_snapshot": forceSnapshot,
	}
	var result CompactResult
	if err := c.doRequest(ctx, opCompact, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
