package rstmdb

import (
	"context"
	"encoding/base64"
)

// Data decodes the base64-encoded backup chunk payload.
func (r *BackupChunkResult) Data() ([]byte, error) {
	if r == nil || r.Bytes == "" {
		return nil, nil
	}
	return base64.StdEncoding.DecodeString(r.Bytes)
}

// BackupBegin starts a streaming hot backup on the server.
func (c *Client) BackupBegin(ctx context.Context, opts *BackupBeginOptions) (*BackupBeginResult, error) {
	var params any = map[string]any{}
	if opts != nil {
		params = opts
	}
	var result BackupBeginResult
	if err := c.doRequest(ctx, opBackupBegin, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// BackupChunk reads a chunk of a staged backup archive.
func (c *Client) BackupChunk(ctx context.Context, backupID string, offset, length uint64) (*BackupChunkResult, error) {
	params := map[string]any{
		"backup_id": backupID,
		"offset":    offset,
		"len":       length,
	}
	var result BackupChunkResult
	if err := c.doRequest(ctx, opBackupChunk, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// BackupEnd finalizes and cleans up a staged backup on the server.
func (c *Client) BackupEnd(ctx context.Context, backupID string) (*BackupEndResult, error) {
	params := map[string]any{
		"backup_id": backupID,
	}
	var result BackupEndResult
	if err := c.doRequest(ctx, opBackupEnd, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
