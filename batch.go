package rstmdb

import "context"

// Batch executes multiple operations in a single request.
func (c *Client) Batch(ctx context.Context, mode BatchMode, ops []BatchOperation) ([]BatchResult, error) {
	params := map[string]any{
		"mode": mode,
		"ops":  ops,
	}
	var result struct {
		Results []BatchResult `json:"results"`
	}
	if err := c.doRequest(ctx, opBatch, params, &result); err != nil {
		return nil, err
	}
	return result.Results, nil
}

// BatchCreateInstance builds a BatchOperation for CREATE_INSTANCE.
func BatchCreateInstance(req CreateInstanceRequest) BatchOperation {
	return BatchOperation{Op: string(opCreateInstance), Params: req}
}

// BatchApplyEvent builds a BatchOperation for APPLY_EVENT.
func BatchApplyEvent(req ApplyEventRequest) BatchOperation {
	return BatchOperation{Op: string(opApplyEvent), Params: req}
}

// BatchDeleteInstance builds a BatchOperation for DELETE_INSTANCE.
func BatchDeleteInstance(instanceID string) BatchOperation {
	return BatchOperation{
		Op:     string(opDeleteInstance),
		Params: map[string]any{"instance_id": instanceID},
	}
}
