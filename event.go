package rstmdb

import "context"

// ApplyEvent applies an event to transition an instance.
func (c *Client) ApplyEvent(ctx context.Context, req ApplyEventRequest) (*ApplyEventResult, error) {
	var result ApplyEventResult
	if err := c.doRequest(ctx, opApplyEvent, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
