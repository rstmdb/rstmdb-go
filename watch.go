package rstmdb

import (
	"context"
	"encoding/json"
	"sync/atomic"
)

// Subscription represents a server-pushed event stream.
type Subscription struct {
	ID     string
	Events <-chan Event
	Errors <-chan error

	events chan Event
	errors chan error
	client *Client
	closed atomic.Bool
}

func newSubscription(client *Client) *Subscription {
	events := make(chan Event, 256)
	errors := make(chan error, 256)
	return &Subscription{
		Events: events,
		Errors: errors,
		events: events,
		errors: errors,
		client: client,
	}
}

// Close cancels the subscription by sending UNWATCH to the server.
func (s *Subscription) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}

	// Remove from conn's subscription map
	s.client.conn.subsMu.Lock()
	delete(s.client.conn.subs, s.ID)
	s.client.conn.subsMu.Unlock()

	// Best-effort UNWATCH
	params := map[string]any{"subscription_id": s.ID}
	_ = s.client.doRequest(context.Background(), opUnwatch, params, nil)

	s.closeChannels()
	return nil
}

func (s *Subscription) closeChannels() {
	// Safe to call multiple times due to closed flag, but guard channels
	if s.closed.CompareAndSwap(false, true) {
		close(s.events)
		close(s.errors)
		return
	}
	// If already closed via Close(), channels may already be closed
	// Use recover to avoid panic on double close
	defer func() { recover() }()
	close(s.events)
	close(s.errors)
}

// WatchInstance subscribes to state changes for a single instance.
func (c *Client) WatchInstance(ctx context.Context, req WatchInstanceRequest) (*Subscription, error) {
	sub := newSubscription(c)

	// Register sub BEFORE sending request to avoid race
	// We'll set the real ID after the response
	resp, err := c.conn.sendAndWait(ctx, opWatchInstance, req)
	if err != nil {
		return nil, err
	}

	if resp.Status == "error" && resp.Error != nil {
		return nil, resp.Error
	}

	var result struct {
		SubscriptionID string `json:"subscription_id"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}

	sub.ID = result.SubscriptionID
	c.conn.subsMu.Lock()
	c.conn.subs[sub.ID] = sub
	c.conn.subsMu.Unlock()

	return sub, nil
}

// WatchAll subscribes to all instance events with optional filtering.
func (c *Client) WatchAll(ctx context.Context, opts ...WatchOption) (*Subscription, error) {
	params := map[string]any{}
	for _, opt := range opts {
		opt(params)
	}

	sub := newSubscription(c)

	resp, err := c.conn.sendAndWait(ctx, opWatchAll, params)
	if err != nil {
		return nil, err
	}

	if resp.Status == "error" && resp.Error != nil {
		return nil, resp.Error
	}

	var result struct {
		SubscriptionID string `json:"subscription_id"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, err
	}

	sub.ID = result.SubscriptionID
	c.conn.subsMu.Lock()
	c.conn.subs[sub.ID] = sub
	c.conn.subsMu.Unlock()

	return sub, nil
}
