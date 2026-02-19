package rstmdb

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type conn struct {
	netConn   net.Conn
	writer    *bufio.Writer
	writeMu   sync.Mutex
	reader    *bufio.Reader
	wireMode  string // "binary_json" or "jsonl"
	pending   map[string]chan *rawResponse
	pendingMu sync.Mutex
	subs      map[string]*Subscription
	subsMu    sync.Mutex
	nextID    atomic.Uint64
	done      chan struct{}
	closeOnce sync.Once

	requestTimeout time.Duration
}

func dial(ctx context.Context, addr string, opts *Options) (*conn, error) {
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	dialer := &net.Dialer{Timeout: timeout}
	var netConn net.Conn
	var err error

	if opts.TLS != nil {
		netConn, err = tls.DialWithDialer(dialer, "tcp", addr, opts.TLS)
	} else {
		netConn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return nil, fmt.Errorf("rstmdb: dial %s: %w", addr, err)
	}

	if tc, ok := netConn.(*net.TCPConn); ok {
		_ = tc.SetNoDelay(true)
	}

	reqTimeout := opts.RequestTimeout
	if reqTimeout == 0 {
		reqTimeout = 30 * time.Second
	}

	c := &conn{
		netConn:        netConn,
		writer:         bufio.NewWriter(netConn),
		reader:         bufio.NewReaderSize(netConn, 8192),
		wireMode:       "binary_json",
		pending:        make(map[string]chan *rawResponse),
		subs:           make(map[string]*Subscription),
		done:           make(chan struct{}),
		requestTimeout: reqTimeout,
	}

	return c, nil
}

func (c *conn) nextRequestID() string {
	id := c.nextID.Add(1)
	return fmt.Sprintf("%d", id)
}

func (c *conn) sendRequest(req *request) error {
	var data []byte
	var err error

	if c.wireMode == "jsonl" {
		data, err = encodeJSONL(req)
	} else {
		payload, merr := json.Marshal(req)
		if merr != nil {
			return merr
		}
		data, err = encodeFrame(payload)
	}
	if err != nil {
		return err
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if _, err := c.writer.Write(data); err != nil {
		return fmt.Errorf("%w: %v", ErrConnectionClosed, err)
	}
	return c.writer.Flush()
}

func (c *conn) sendAndWait(ctx context.Context, op operation, params any) (*rawResponse, error) {
	id := c.nextRequestID()
	req := &request{
		Type:   "request",
		ID:     id,
		Op:     string(op),
		Params: params,
	}

	ch := make(chan *rawResponse, 1)
	c.pendingMu.Lock()
	c.pending[id] = ch
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
	}()

	if err := c.sendRequest(req); err != nil {
		return nil, err
	}

	// Apply request timeout
	reqCtx := ctx
	if c.requestTimeout > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, c.requestTimeout)
		defer cancel()
	}

	select {
	case resp := <-ch:
		if resp == nil {
			return nil, ErrConnectionClosed
		}
		return resp, nil
	case <-reqCtx.Done():
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, ErrTimeout
	case <-c.done:
		return nil, ErrConnectionClosed
	}
}

func (c *conn) readLoop() {
	for {
		payload, err := c.readMessage()
		if err != nil {
			select {
			case <-c.done:
				return
			default:
			}
			// Connection error — close everything
			c.close()
			return
		}

		var msg rawMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "response":
			var resp rawResponse
			if err := json.Unmarshal(payload, &resp); err != nil {
				continue
			}
			c.pendingMu.Lock()
			ch, ok := c.pending[resp.ID]
			c.pendingMu.Unlock()
			if ok {
				select {
				case ch <- &resp:
				default:
				}
			}

		case "event":
			var evt Event
			if err := json.Unmarshal(payload, &evt); err != nil {
				continue
			}
			c.subsMu.Lock()
			sub, ok := c.subs[evt.SubscriptionID]
			c.subsMu.Unlock()
			if ok {
				// Non-blocking send; drop if full (cap=256)
				select {
				case sub.events <- evt:
				default:
				}
			}
		}
	}
}

func (c *conn) readMessage() ([]byte, error) {
	if c.wireMode == "jsonl" {
		line, err := c.reader.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		return line, nil
	}
	return decodeFrame(c.reader)
}

func (c *conn) close() error {
	var err error
	c.closeOnce.Do(func() {
		// Best-effort BYE
		bye := &request{Type: "request", ID: c.nextRequestID(), Op: string(opBye)}
		_ = c.sendRequest(bye)

		close(c.done)
		err = c.netConn.Close()

		// Drain pending requests
		c.pendingMu.Lock()
		for id, ch := range c.pending {
			close(ch)
			delete(c.pending, id)
		}
		c.pendingMu.Unlock()

		// Close subscription channels
		c.subsMu.Lock()
		for id, sub := range c.subs {
			sub.closeFromConn()
			delete(c.subs, id)
		}
		c.subsMu.Unlock()
	})
	return err
}
