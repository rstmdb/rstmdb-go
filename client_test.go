package rstmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

// mockServer is a lightweight TCP server that speaks the RCPX binary protocol.
type mockServer struct {
	listener net.Listener
	handler  func(conn net.Conn, req *request) *rawResponse
	mu       sync.Mutex
	conns    []net.Conn
	done     chan struct{}
}

func newMockServer(t *testing.T, handler func(conn net.Conn, req *request) *rawResponse) *mockServer {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	s := &mockServer{
		listener: ln,
		handler:  handler,
		done:     make(chan struct{}),
	}
	go s.serve(t)
	return s
}

func (s *mockServer) addr() string {
	return s.listener.Addr().String()
}

func (s *mockServer) close() {
	close(s.done)
	s.listener.Close()
	s.mu.Lock()
	for _, c := range s.conns {
		c.Close()
	}
	s.mu.Unlock()
}

func (s *mockServer) serve(t *testing.T) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.done:
				return
			default:
			}
			return
		}
		s.mu.Lock()
		s.conns = append(s.conns, conn)
		s.mu.Unlock()
		go s.handleConn(t, conn)
	}
}

func (s *mockServer) handleConn(t *testing.T, conn net.Conn) {
	for {
		payload, err := decodeFrame(conn)
		if err != nil {
			return
		}

		var req request
		if err := json.Unmarshal(payload, &req); err != nil {
			return
		}

		resp := s.handler(conn, &req)
		if resp == nil {
			continue
		}

		respBytes, err := json.Marshal(resp)
		if err != nil {
			return
		}

		frame, err := encodeFrame(respBytes)
		if err != nil {
			return
		}
		if _, err := conn.Write(frame); err != nil {
			return
		}
	}
}

// defaultHandler handles HELLO, AUTH, and returns OK for everything else.
func defaultHandler(resultForOp map[string]any) func(net.Conn, *request) *rawResponse {
	return func(conn net.Conn, req *request) *rawResponse {
		switch req.Op {
		case "HELLO":
			result, _ := json.Marshal(HelloResult{
				ProtocolVersion: 1,
				WireMode:        "binary_json",
				ServerName:      "mock-rstmdb",
				ServerVersion:   "0.1.0",
				Features:        []string{"watch", "batch"},
			})
			return &rawResponse{
				Type:   "response",
				ID:     req.ID,
				Status: "ok",
				Result: result,
			}
		case "AUTH":
			return &rawResponse{
				Type:   "response",
				ID:     req.ID,
				Status: "ok",
				Result: json.RawMessage(`{"authenticated":true}`),
			}
		case "BYE":
			return &rawResponse{
				Type:   "response",
				ID:     req.ID,
				Status: "ok",
			}
		default:
			if resultForOp != nil {
				if r, ok := resultForOp[req.Op]; ok {
					data, _ := json.Marshal(r)
					return &rawResponse{
						Type:   "response",
						ID:     req.ID,
						Status: "ok",
						Result: data,
					}
				}
			}
			return &rawResponse{
				Type:   "response",
				ID:     req.ID,
				Status: "ok",
				Result: json.RawMessage(`{}`),
			}
		}
	}
}

func errorHandler(code ErrorCode, message string) func(net.Conn, *request) *rawResponse {
	return func(conn net.Conn, req *request) *rawResponse {
		switch req.Op {
		case "HELLO":
			result, _ := json.Marshal(HelloResult{
				ProtocolVersion: 1,
				WireMode:        "binary_json",
				ServerName:      "mock-rstmdb",
				ServerVersion:   "0.1.0",
			})
			return &rawResponse{
				Type:   "response",
				ID:     req.ID,
				Status: "ok",
				Result: result,
			}
		case "AUTH":
			return &rawResponse{
				Type:   "response",
				ID:     req.ID,
				Status: "ok",
				Result: json.RawMessage(`{"authenticated":true}`),
			}
		case "BYE":
			return nil
		default:
			return &rawResponse{
				Type:   "response",
				ID:     req.ID,
				Status: "error",
				Error: &Error{
					Code:    code,
					Message: message,
				},
			}
		}
	}
}

func connectToMock(t *testing.T, s *mockServer, opts *Options) *Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := Connect(ctx, s.addr(), opts)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	return client
}

// --- Tests ---

func TestConnect(t *testing.T) {
	s := newMockServer(t, defaultHandler(nil))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	if client.serverInfo.ServerName != "mock-rstmdb" {
		t.Fatalf("server_name: got %q, want mock-rstmdb", client.serverInfo.ServerName)
	}
	if client.serverInfo.ProtocolVersion != 1 {
		t.Fatalf("protocol_version: got %d, want 1", client.serverInfo.ProtocolVersion)
	}
}

func TestConnectWithAuth(t *testing.T) {
	s := newMockServer(t, defaultHandler(nil))
	defer s.close()

	client := connectToMock(t, s, &Options{Auth: "test-token"})
	defer client.Close()
}

func TestPing(t *testing.T) {
	s := newMockServer(t, defaultHandler(map[string]any{
		"PING": map[string]any{"pong": true},
	}))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestInfo(t *testing.T) {
	s := newMockServer(t, defaultHandler(map[string]any{
		"INFO": ServerInfo{
			ServerName:      "rstmdb",
			ServerVersion:   "0.2.0",
			ProtocolVersion: 1,
			Features:        []string{"watch"},
			MaxFrameBytes:   16777216,
			MaxBatchOps:     100,
		},
	}))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	info, err := client.Info(ctx)
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.ServerName != "rstmdb" {
		t.Fatalf("server_name: got %q, want rstmdb", info.ServerName)
	}
	if info.MaxBatchOps != 100 {
		t.Fatalf("max_batch_ops: got %d, want 100", info.MaxBatchOps)
	}
}

func TestPutMachine(t *testing.T) {
	s := newMockServer(t, defaultHandler(map[string]any{
		"PUT_MACHINE": PutMachineResult{
			Machine:        "order",
			Version:        1,
			StoredChecksum: "abc123",
			Created:        true,
		},
	}))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	result, err := client.PutMachine(ctx, PutMachineRequest{
		Machine: "order",
		Version: 1,
		Definition: MachineDefinition{
			States:  []string{"created", "paid"},
			Initial: "created",
			Transitions: []Transition{
				{From: StringOrSlice{"created"}, Event: "PAY", To: "paid"},
			},
		},
	})
	if err != nil {
		t.Fatalf("PutMachine: %v", err)
	}
	if !result.Created {
		t.Fatal("expected Created=true")
	}
	if result.Machine != "order" {
		t.Fatalf("machine: got %q, want order", result.Machine)
	}
}

func TestGetMachine(t *testing.T) {
	s := newMockServer(t, defaultHandler(map[string]any{
		"GET_MACHINE": MachineInfo{
			Definition: MachineDefinition{
				States:  []string{"a", "b"},
				Initial: "a",
			},
			Checksum: "sha256hex",
		},
	}))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	info, err := client.GetMachine(ctx, "order", 1)
	if err != nil {
		t.Fatalf("GetMachine: %v", err)
	}
	if info.Checksum != "sha256hex" {
		t.Fatalf("checksum: got %q, want sha256hex", info.Checksum)
	}
}

func TestListMachines(t *testing.T) {
	s := newMockServer(t, defaultHandler(map[string]any{
		"LIST_MACHINES": map[string]any{
			"items": []MachineSummary{
				{Machine: "order", Versions: []int{1, 2}},
				{Machine: "user", Versions: []int{1}},
			},
		},
	}))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	machines, err := client.ListMachines(ctx)
	if err != nil {
		t.Fatalf("ListMachines: %v", err)
	}
	if len(machines) != 2 {
		t.Fatalf("expected 2 machines, got %d", len(machines))
	}
	if machines[0].Machine != "order" {
		t.Fatalf("machines[0].Machine: got %q, want order", machines[0].Machine)
	}
}

func TestCreateInstance(t *testing.T) {
	s := newMockServer(t, defaultHandler(map[string]any{
		"CREATE_INSTANCE": CreateInstanceResult{
			InstanceID: "inst-1",
			State:      "created",
			WALOffset:  1,
		},
	}))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	result, err := client.CreateInstance(ctx, CreateInstanceRequest{
		Machine: "order",
		Version: 1,
	})
	if err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}
	if result.InstanceID != "inst-1" {
		t.Fatalf("instance_id: got %q, want inst-1", result.InstanceID)
	}
	if result.State != "created" {
		t.Fatalf("state: got %q, want created", result.State)
	}
}

func TestGetInstance(t *testing.T) {
	s := newMockServer(t, defaultHandler(map[string]any{
		"GET_INSTANCE": Instance{
			Machine: "order",
			Version: 1,
			State:   "paid",
			Ctx:     map[string]any{"amount": float64(99)},
		},
	}))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	inst, err := client.GetInstance(ctx, "inst-1")
	if err != nil {
		t.Fatalf("GetInstance: %v", err)
	}
	if inst.State != "paid" {
		t.Fatalf("state: got %q, want paid", inst.State)
	}
}

func TestListInstances(t *testing.T) {
	s := newMockServer(t, defaultHandler(map[string]any{
		"LIST_INSTANCES": InstanceList{
			Instances: []InstanceSummary{
				{ID: "i1", Machine: "order", State: "created"},
				{ID: "i2", Machine: "order", State: "paid"},
			},
			Total:   2,
			HasMore: false,
		},
	}))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	list, err := client.ListInstances(ctx, WithMachine("order"), WithLimit(10))
	if err != nil {
		t.Fatalf("ListInstances: %v", err)
	}
	if list.Total != 2 {
		t.Fatalf("total: got %d, want 2", list.Total)
	}
	if len(list.Instances) != 2 {
		t.Fatalf("instances: got %d, want 2", len(list.Instances))
	}
}

func TestDeleteInstance(t *testing.T) {
	s := newMockServer(t, defaultHandler(map[string]any{
		"DELETE_INSTANCE": DeleteInstanceResult{
			InstanceID: "inst-1",
			Deleted:    true,
			WALOffset:  5,
		},
	}))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	result, err := client.DeleteInstance(ctx, "inst-1", WithIdempotencyKey("key-1"))
	if err != nil {
		t.Fatalf("DeleteInstance: %v", err)
	}
	if !result.Deleted {
		t.Fatal("expected Deleted=true")
	}
}

func TestApplyEvent(t *testing.T) {
	s := newMockServer(t, defaultHandler(map[string]any{
		"APPLY_EVENT": ApplyEventResult{
			FromState: "created",
			ToState:   "paid",
			Applied:   true,
			WALOffset: 2,
		},
	}))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	result, err := client.ApplyEvent(ctx, ApplyEventRequest{
		InstanceID: "inst-1",
		Event:      "PAY",
		Payload:    map[string]any{"amount": 99.99},
	})
	if err != nil {
		t.Fatalf("ApplyEvent: %v", err)
	}
	if !result.Applied {
		t.Fatal("expected Applied=true")
	}
	if result.ToState != "paid" {
		t.Fatalf("to_state: got %q, want paid", result.ToState)
	}
}

func TestBatch(t *testing.T) {
	s := newMockServer(t, defaultHandler(map[string]any{
		"BATCH": map[string]any{
			"results": []map[string]any{
				{"status": "ok", "result": map[string]any{"instance_id": "i1"}},
				{"status": "ok", "result": map[string]any{"applied": true}},
			},
		},
	}))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	ops := []BatchOperation{
		BatchCreateInstance(CreateInstanceRequest{Machine: "order", Version: 1}),
		BatchApplyEvent(ApplyEventRequest{InstanceID: "i1", Event: "PAY"}),
	}
	results, err := client.Batch(ctx, BatchAtomic, ops)
	if err != nil {
		t.Fatalf("Batch: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Status != "ok" {
		t.Fatalf("results[0].Status: got %q, want ok", results[0].Status)
	}
}

func TestBatchHelpers(t *testing.T) {
	op1 := BatchCreateInstance(CreateInstanceRequest{Machine: "m", Version: 1})
	if op1.Op != "CREATE_INSTANCE" {
		t.Fatalf("op: got %q, want CREATE_INSTANCE", op1.Op)
	}

	op2 := BatchApplyEvent(ApplyEventRequest{InstanceID: "i1", Event: "E"})
	if op2.Op != "APPLY_EVENT" {
		t.Fatalf("op: got %q, want APPLY_EVENT", op2.Op)
	}

	op3 := BatchDeleteInstance("i1")
	if op3.Op != "DELETE_INSTANCE" {
		t.Fatalf("op: got %q, want DELETE_INSTANCE", op3.Op)
	}
}

func TestSnapshotInstance(t *testing.T) {
	s := newMockServer(t, defaultHandler(map[string]any{
		"SNAPSHOT_INSTANCE": SnapshotResult{
			InstanceID: "inst-1",
			SnapshotID: "snap-1",
			WALOffset:  10,
			SizeBytes:  4096,
		},
	}))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	result, err := client.SnapshotInstance(ctx, "inst-1")
	if err != nil {
		t.Fatalf("SnapshotInstance: %v", err)
	}
	if result.SnapshotID != "snap-1" {
		t.Fatalf("snapshot_id: got %q, want snap-1", result.SnapshotID)
	}
}

func TestWALRead(t *testing.T) {
	s := newMockServer(t, defaultHandler(map[string]any{
		"WAL_READ": WALReadResult{
			Records: []WALRecord{
				{Sequence: 1, Offset: 0, Entry: map[string]any{"type": "create"}},
				{Sequence: 2, Offset: 1, Entry: map[string]any{"type": "event"}},
			},
			NextOffset: 2,
		},
	}))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	result, err := client.WALRead(ctx, 0, WALLimit(10))
	if err != nil {
		t.Fatalf("WALRead: %v", err)
	}
	if len(result.Records) != 2 {
		t.Fatalf("records: got %d, want 2", len(result.Records))
	}
	if result.NextOffset != 2 {
		t.Fatalf("next_offset: got %d, want 2", result.NextOffset)
	}
}

func TestWALStats(t *testing.T) {
	s := newMockServer(t, defaultHandler(map[string]any{
		"WAL_STATS": WALStatsResult{
			EntryCount:   100,
			SegmentCount: 3,
			TotalSize:    1024000,
			LatestOffset: 99,
			IOStats: IOStats{
				BytesWritten: 500000,
				BytesRead:    200000,
				Writes:       100,
				Reads:        50,
				Fsyncs:       100,
			},
		},
	}))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	stats, err := client.WALStats(ctx)
	if err != nil {
		t.Fatalf("WALStats: %v", err)
	}
	if stats.EntryCount != 100 {
		t.Fatalf("entry_count: got %d, want 100", stats.EntryCount)
	}
	if stats.IOStats.Fsyncs != 100 {
		t.Fatalf("fsyncs: got %d, want 100", stats.IOStats.Fsyncs)
	}
}

func TestCompact(t *testing.T) {
	s := newMockServer(t, defaultHandler(map[string]any{
		"COMPACT": CompactResult{
			SnapshotsCreated: 2,
			SegmentsDeleted:  5,
			BytesReclaimed:   1048576,
			TotalSnapshots:   10,
			WALSegments:      3,
		},
	}))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	result, err := client.Compact(ctx, true)
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if result.SnapshotsCreated != 2 {
		t.Fatalf("snapshots_created: got %d, want 2", result.SnapshotsCreated)
	}
	if result.BytesReclaimed != 1048576 {
		t.Fatalf("bytes_reclaimed: got %d, want 1048576", result.BytesReclaimed)
	}
}

func TestFlushAll(t *testing.T) {
	s := newMockServer(t, defaultHandler(map[string]any{
		"FLUSH_ALL": FlushAllResult{
			Flushed:          true,
			InstancesRemoved: 42,
			MachinesRemoved:  5,
		},
	}))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	result, err := client.FlushAll(ctx)
	if err != nil {
		t.Fatalf("FlushAll: %v", err)
	}
	if !result.Flushed {
		t.Fatal("expected Flushed=true")
	}
	if result.InstancesRemoved != 42 {
		t.Fatalf("instances_removed: got %d, want 42", result.InstancesRemoved)
	}
	if result.MachinesRemoved != 5 {
		t.Fatalf("machines_removed: got %d, want 5", result.MachinesRemoved)
	}
}

func TestWatchInstance(t *testing.T) {
	subID := "sub-watch-1"
	s := newMockServer(t, func(conn net.Conn, req *request) *rawResponse {
		switch req.Op {
		case "HELLO":
			result, _ := json.Marshal(HelloResult{
				ProtocolVersion: 1,
				WireMode:        "binary_json",
				ServerName:      "mock",
				ServerVersion:   "0.1.0",
			})
			return &rawResponse{Type: "response", ID: req.ID, Status: "ok", Result: result}
		case "AUTH":
			return &rawResponse{Type: "response", ID: req.ID, Status: "ok", Result: json.RawMessage(`{}`)}
		case "WATCH_INSTANCE":
			result, _ := json.Marshal(map[string]any{"subscription_id": subID})
			resp := &rawResponse{Type: "response", ID: req.ID, Status: "ok", Result: result}

			// Send an event after a brief delay
			go func() {
				time.Sleep(50 * time.Millisecond)
				evtMap := map[string]any{
					"type":            "event",
					"subscription_id": subID,
					"instance_id":     "inst-1",
					"machine":         "order",
					"version":         1,
					"from_state":      "created",
					"to_state":        "paid",
					"event":           "PAY",
				}
				data, _ := json.Marshal(evtMap)
				frame, _ := encodeFrame(data)
				conn.Write(frame)
			}()

			return resp
		case "UNWATCH":
			return &rawResponse{Type: "response", ID: req.ID, Status: "ok", Result: json.RawMessage(`{}`)}
		case "BYE":
			return nil
		default:
			return &rawResponse{Type: "response", ID: req.ID, Status: "ok", Result: json.RawMessage(`{}`)}
		}
	})
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	sub, err := client.WatchInstance(ctx, WatchInstanceRequest{
		InstanceID: "inst-1",
		IncludeCtx: true,
	})
	if err != nil {
		t.Fatalf("WatchInstance: %v", err)
	}
	defer sub.Close()

	if sub.ID != subID {
		t.Fatalf("subscription_id: got %q, want %q", sub.ID, subID)
	}

	select {
	case evt := <-sub.Events:
		if evt.ToState != "paid" {
			t.Fatalf("event.to_state: got %q, want paid", evt.ToState)
		}
		if evt.InstanceID != "inst-1" {
			t.Fatalf("event.instance_id: got %q, want inst-1", evt.InstanceID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestWatchAll(t *testing.T) {
	subID := "sub-all-1"
	s := newMockServer(t, func(conn net.Conn, req *request) *rawResponse {
		switch req.Op {
		case "HELLO":
			result, _ := json.Marshal(HelloResult{
				ProtocolVersion: 1,
				WireMode:        "binary_json",
				ServerName:      "mock",
				ServerVersion:   "0.1.0",
			})
			return &rawResponse{Type: "response", ID: req.ID, Status: "ok", Result: result}
		case "WATCH_ALL":
			result, _ := json.Marshal(map[string]any{"subscription_id": subID})
			return &rawResponse{Type: "response", ID: req.ID, Status: "ok", Result: result}
		case "UNWATCH":
			return &rawResponse{Type: "response", ID: req.ID, Status: "ok", Result: json.RawMessage(`{}`)}
		case "BYE":
			return nil
		default:
			return &rawResponse{Type: "response", ID: req.ID, Status: "ok", Result: json.RawMessage(`{}`)}
		}
	})
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	sub, err := client.WatchAll(ctx,
		WatchMachines("order"),
		WatchToStates("shipped"),
	)
	if err != nil {
		t.Fatalf("WatchAll: %v", err)
	}
	defer sub.Close()

	if sub.ID != subID {
		t.Fatalf("subscription_id: got %q, want %q", sub.ID, subID)
	}
}

// --- Error handling tests ---

func TestErrorNotFound(t *testing.T) {
	s := newMockServer(t, errorHandler(ErrCodeNotFound, "not found"))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	_, err := client.GetInstance(ctx, "no-such-id")
	if err == nil {
		t.Fatal("expected error")
	}

	if !IsNotFound(err) {
		t.Fatalf("expected IsNotFound, got %v", err)
	}
}

func TestErrorInstanceNotFound(t *testing.T) {
	s := newMockServer(t, errorHandler(ErrCodeInstanceNotFound, "instance not found"))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	_, err := client.GetInstance(ctx, "no-such-id")
	if !IsInstanceNotFound(err) {
		t.Fatalf("expected IsInstanceNotFound, got %v", err)
	}
}

func TestErrorMachineNotFound(t *testing.T) {
	s := newMockServer(t, errorHandler(ErrCodeMachineNotFound, "machine not found"))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	_, err := client.GetMachine(ctx, "no-such", 1)
	if !IsMachineNotFound(err) {
		t.Fatalf("expected IsMachineNotFound, got %v", err)
	}
}

func TestErrorInvalidTransition(t *testing.T) {
	s := newMockServer(t, errorHandler(ErrCodeInvalidTransition, "invalid transition"))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	_, err := client.ApplyEvent(ctx, ApplyEventRequest{InstanceID: "i1", Event: "BAD"})
	if !IsInvalidTransition(err) {
		t.Fatalf("expected IsInvalidTransition, got %v", err)
	}
}

func TestErrorGuardFailed(t *testing.T) {
	s := newMockServer(t, errorHandler(ErrCodeGuardFailed, "guard failed"))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	_, err := client.ApplyEvent(ctx, ApplyEventRequest{InstanceID: "i1", Event: "E"})
	if !IsGuardFailed(err) {
		t.Fatalf("expected IsGuardFailed, got %v", err)
	}
}

func TestErrorConflict(t *testing.T) {
	s := newMockServer(t, errorHandler(ErrCodeConflict, "conflict"))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	_, err := client.ApplyEvent(ctx, ApplyEventRequest{InstanceID: "i1", Event: "E"})
	if !IsConflict(err) {
		t.Fatalf("expected IsConflict, got %v", err)
	}
}

func TestErrorRetryable(t *testing.T) {
	s := newMockServer(t, func(conn net.Conn, req *request) *rawResponse {
		switch req.Op {
		case "HELLO":
			result, _ := json.Marshal(HelloResult{
				ProtocolVersion: 1,
				WireMode:        "binary_json",
				ServerName:      "mock",
				ServerVersion:   "0.1.0",
			})
			return &rawResponse{Type: "response", ID: req.ID, Status: "ok", Result: result}
		case "BYE":
			return nil
		default:
			return &rawResponse{
				Type:   "response",
				ID:     req.ID,
				Status: "error",
				Error: &Error{
					Code:      ErrCodeRateLimited,
					Message:   "slow down",
					Retryable: true,
				},
			}
		}
	})
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	err := client.Ping(ctx)
	if !IsRetryable(err) {
		t.Fatalf("expected IsRetryable, got %v", err)
	}
}

func TestErrorString(t *testing.T) {
	e := &Error{Code: ErrCodeBadRequest, Message: "missing field"}
	expected := "rstmdb: BAD_REQUEST: missing field"
	if e.Error() != expected {
		t.Fatalf("Error(): got %q, want %q", e.Error(), expected)
	}

	e2 := &Error{Code: ErrCodeInternalError}
	expected2 := "rstmdb: INTERNAL_ERROR"
	if e2.Error() != expected2 {
		t.Fatalf("Error(): got %q, want %q", e2.Error(), expected2)
	}
}

func TestErrorAs(t *testing.T) {
	err := fmt.Errorf("wrapped: %w", &Error{Code: ErrCodeConflict, Message: "test"})
	var rstmdbErr *Error
	if !errors.As(err, &rstmdbErr) {
		t.Fatal("expected errors.As to match *Error")
	}
	if rstmdbErr.Code != ErrCodeConflict {
		t.Fatalf("code: got %q, want CONFLICT", rstmdbErr.Code)
	}
}

func TestContextCancellation(t *testing.T) {
	s := newMockServer(t, func(conn net.Conn, req *request) *rawResponse {
		switch req.Op {
		case "HELLO":
			result, _ := json.Marshal(HelloResult{
				ProtocolVersion: 1,
				WireMode:        "binary_json",
				ServerName:      "mock",
				ServerVersion:   "0.1.0",
			})
			return &rawResponse{Type: "response", ID: req.ID, Status: "ok", Result: result}
		case "BYE":
			return nil
		default:
			return nil
		}
	})
	defer s.close()

	client := connectToMock(t, s, &Options{RequestTimeout: 10 * time.Second})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := client.Ping(ctx)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
}

func TestConnectionClose(t *testing.T) {
	s := newMockServer(t, defaultHandler(nil))
	defer s.close()

	client := connectToMock(t, s, nil)

	client.Close()

	ctx := context.Background()
	err := client.Ping(ctx)
	if err == nil {
		t.Fatal("expected error after close")
	}
}

func TestAllErrorCodes(t *testing.T) {
	codes := []ErrorCode{
		ErrCodeUnsupportedProtocol,
		ErrCodeBadRequest,
		ErrCodeUnauthorized,
		ErrCodeAuthFailed,
		ErrCodeNotFound,
		ErrCodeMachineNotFound,
		ErrCodeMachineVersionExists,
		ErrCodeMachineVersionLimitExceeded,
		ErrCodeInstanceNotFound,
		ErrCodeInstanceExists,
		ErrCodeInvalidTransition,
		ErrCodeGuardFailed,
		ErrCodeConflict,
		ErrCodeWALIOError,
		ErrCodeInternalError,
		ErrCodeRateLimited,
	}

	if len(codes) != 16 {
		t.Fatalf("expected 16 error codes, got %d", len(codes))
	}

	seen := make(map[ErrorCode]bool)
	for _, code := range codes {
		if code == "" {
			t.Fatal("empty error code")
		}
		if seen[code] {
			t.Fatalf("duplicate error code: %s", code)
		}
		seen[code] = true
	}
}

func TestConcurrentRequests(t *testing.T) {
	s := newMockServer(t, defaultHandler(map[string]any{
		"PING": map[string]any{"pong": true},
	}))
	defer s.close()

	client := connectToMock(t, s, nil)
	defer client.Close()

	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := client.Ping(ctx); err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("concurrent Ping failed: %v", err)
	}
}
