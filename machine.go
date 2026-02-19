package rstmdb

import "context"

// PutMachine registers or updates a state machine definition.
func (c *Client) PutMachine(ctx context.Context, req PutMachineRequest) (*PutMachineResult, error) {
	var result PutMachineResult
	if err := c.doRequest(ctx, opPutMachine, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetMachine retrieves a machine definition by name and version.
func (c *Client) GetMachine(ctx context.Context, machine string, version int) (*MachineInfo, error) {
	params := map[string]any{
		"machine": machine,
		"version": version,
	}
	var result MachineInfo
	if err := c.doRequest(ctx, opGetMachine, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// ListMachines lists all registered machines.
func (c *Client) ListMachines(ctx context.Context) ([]MachineSummary, error) {
	var result struct {
		Items []MachineSummary `json:"items"`
	}
	if err := c.doRequest(ctx, opListMachines, nil, &result); err != nil {
		return nil, err
	}
	return result.Items, nil
}
