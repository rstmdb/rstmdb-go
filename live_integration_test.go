package rstmdb

import (
	"context"
	"fmt"
	"net"
	"os"
	"slices"
	"testing"
	"time"
)

func TestLiveServerParity(t *testing.T) {
	host := os.Getenv("RSTMDB_HOST")
	if host == "" {
		t.Skip("RSTMDB_HOST not set; skipping live-server integration test")
	}
	port := os.Getenv("RSTMDB_PORT")
	if port == "" {
		port = "7401"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client, err := Connect(ctx, net.JoinHostPort(host, port), &Options{
		Auth:       os.Getenv("RSTMDB_TOKEN"),
		ClientName: "rstmdb-go-live-test",
	})
	if err != nil {
		t.Fatalf("Connect live server: %v", err)
	}
	defer client.Close()

	if !slices.Contains(client.NegotiatedFeatures(), "durable_idempotency") {
		t.Fatalf("expected durable_idempotency in NegotiatedFeatures, got %v", client.NegotiatedFeatures())
	}

	rep, err := client.ReplicationStatus(ctx)
	if err != nil {
		t.Fatalf("ReplicationStatus: %v", err)
	}
	if rep.Role == "" {
		t.Fatal("expected non-empty replication role")
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	machineName := "go_live_" + suffix

	putRes, err := client.PutMachine(ctx, PutMachineRequest{
		Machine: machineName,
		Version: 1,
		Definition: MachineDefinition{
			States:  []string{"pending", "paid", "shipped"},
			Initial: "pending",
			Transitions: []Transition{
				{From: StringOrSlice{"pending"}, Event: "PAY", To: "paid"},
				{From: StringOrSlice{"paid"}, Event: "SHIP", To: "shipped"},
			},
		},
	})
	if err != nil || !putRes.Created {
		t.Fatalf("PutMachine: res=%#v err=%v", putRes, err)
	}

	instID := "go-inst-" + suffix
	createKey := "go-ck-" + suffix
	cr1, err := client.CreateInstance(ctx, CreateInstanceRequest{
		InstanceID:     instID,
		Machine:        machineName,
		Version:        1,
		InitialCtx:     map[string]any{"customer": "alice"},
		IdempotencyKey: createKey,
	})
	if err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}

	cr2, err := client.CreateInstance(ctx, CreateInstanceRequest{
		InstanceID:     instID,
		Machine:        machineName,
		Version:        1,
		InitialCtx:     map[string]any{"customer": "alice"},
		IdempotencyKey: createKey,
	})
	if err != nil || cr2.WALOffset != cr1.WALOffset {
		t.Fatalf("CreateInstance replay: cr2=%#v err=%v", cr2, err)
	}

	payKey := "go-pk-" + suffix
	ap1, err := client.ApplyEvent(ctx, ApplyEventRequest{
		InstanceID:     instID,
		Event:          "PAY",
		Payload:        map[string]any{"amount": 42},
		IdempotencyKey: payKey,
	})
	if err != nil || !ap1.Applied || ap1.ToState != "paid" {
		t.Fatalf("ApplyEvent: res=%#v err=%v", ap1, err)
	}

	ap2, err := client.ApplyEvent(ctx, ApplyEventRequest{
		InstanceID:     instID,
		Event:          "PAY",
		Payload:        map[string]any{"amount": 42},
		IdempotencyKey: payKey,
	})
	if err != nil || ap2.Applied || ap2.WALOffset != ap1.WALOffset {
		t.Fatalf("ApplyEvent replay: res=%#v err=%v", ap2, err)
	}

	ap3, err := client.ApplyEvent(ctx, ApplyEventRequest{
		InstanceID:     instID,
		Event:          "SHIP",
		IdempotencyKey: payKey,
	})
	if err != nil {
		if !IsIdempotencyConflict(err) || IsRetryable(err) {
			t.Fatalf("expected non-retryable IDEMPOTENCY_CONFLICT, got %v", err)
		}
	} else if ap3.Applied || ap3.WALOffset != ap1.WALOffset {
		t.Fatalf("expected either IDEMPOTENCY_CONFLICT or cached replay on pre-#40 server, got %#v", ap3)
	} else {
		t.Log("target server does not yet enforce IDEMPOTENCY_CONFLICT (pre-rstmdb#40); observed legacy cached replay")
	}

	bkp, err := client.BackupBegin(ctx, &BackupBeginOptions{Compression: "gzip"})
	if err != nil {
		t.Fatalf("BackupBegin: %v", err)
	}
	var downloaded uint64
	for {
		chunk, err := client.BackupChunk(ctx, bkp.BackupID, downloaded, 1024*1024)
		if err != nil {
			t.Fatalf("BackupChunk: %v", err)
		}
		raw, err := chunk.Data()
		if err != nil {
			t.Fatalf("BackupChunk.Data: %v", err)
		}
		downloaded += uint64(len(raw))
		if chunk.EOF {
			break
		}
	}
	if downloaded != bkp.TotalBytes {
		t.Fatalf("backup downloaded %d bytes != total_bytes %d", downloaded, bkp.TotalBytes)
	}
	endRes, err := client.BackupEnd(ctx, bkp.BackupID)
	if err != nil || !endRes.OK {
		t.Fatalf("BackupEnd: res=%#v err=%v", endRes, err)
	}
}
