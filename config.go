package rstmdb

import (
	"crypto/tls"
	"time"
)

// Options configures the client connection. Zero values are ignored.
type Options struct {
	// Auth is the bearer token for authentication. Empty means no auth.
	Auth string

	// TLS is a custom TLS configuration. Nil means plain TCP.
	TLS *tls.Config

	// Timeout is the connection dial timeout. Default: 10s.
	Timeout time.Duration

	// RequestTimeout is the per-request timeout. Default: 30s.
	RequestTimeout time.Duration

	// ClientName is sent in the HELLO handshake.
	ClientName string

	// WireMode is the preferred wire mode: "binary_json" (default) or "jsonl".
	WireMode string

	// Features are feature negotiation hints sent in HELLO.
	Features []string
}

// TLSFromFiles is a convenience to build Options.TLS from PEM files.
// Returns an error if the files cannot be loaded.
func TLSFromFiles(caFile, certFile, keyFile string) (*tls.Config, error) {
	return buildTLSConfig(caFile, certFile, keyFile)
}

// InsecureTLS returns a TLS config that skips certificate verification.
// For development use only.
func InsecureTLS() *tls.Config {
	return buildInsecureTLSConfig()
}

// ListOption configures LIST_INSTANCES.
type ListOption func(map[string]any)

// WithMachine filters instances by machine name.
func WithMachine(name string) ListOption {
	return func(p map[string]any) { p["machine"] = name }
}

// WithState filters instances by state.
func WithState(state string) ListOption {
	return func(p map[string]any) { p["state"] = state }
}

// WithLimit sets the max number of results.
func WithLimit(n int) ListOption {
	return func(p map[string]any) { p["limit"] = n }
}

// WithOffset sets the result offset for pagination.
func WithOffset(n int) ListOption {
	return func(p map[string]any) { p["offset"] = n }
}

// DeleteOption configures DELETE_INSTANCE.
type DeleteOption func(map[string]any)

// WithIdempotencyKey sets an idempotency key for the delete operation.
func WithIdempotencyKey(key string) DeleteOption {
	return func(p map[string]any) { p["idempotency_key"] = key }
}

// WatchOption configures WATCH_ALL.
type WatchOption func(map[string]any)

// WatchIncludeCtx controls whether context is included in events.
func WatchIncludeCtx(include bool) WatchOption {
	return func(p map[string]any) { p["include_ctx"] = include }
}

// WatchFromOffset replays events from a WAL offset.
func WatchFromOffset(offset uint64) WatchOption {
	return func(p map[string]any) { p["from_offset"] = offset }
}

// WatchMachines filters events by machine names.
func WatchMachines(names ...string) WatchOption {
	return func(p map[string]any) { p["machines"] = names }
}

// WatchEvents filters events by event names.
func WatchEvents(events ...string) WatchOption {
	return func(p map[string]any) { p["events"] = events }
}

// WatchFromStates filters events by from_state.
func WatchFromStates(states ...string) WatchOption {
	return func(p map[string]any) { p["from_states"] = states }
}

// WatchToStates filters events by to_state.
func WatchToStates(states ...string) WatchOption {
	return func(p map[string]any) { p["to_states"] = states }
}

// WALReadOption configures WAL_READ.
type WALReadOption func(map[string]any)

// WALLimit sets the max number of WAL records to return.
func WALLimit(n int) WALReadOption {
	return func(p map[string]any) { p["limit"] = n }
}
