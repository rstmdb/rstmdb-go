// Example: WAL and snapshot operations with the rstmdb Go client.
//
// Demonstrates WAL read, WAL stats, snapshot creation, and compaction.
//
// Usage:
//
//	go run ./examples/wal -addr localhost:7401 [-token my-secret]
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	rstmdb "github.com/rstmdb/rstmdb-go"
)

func main() {
	addr := flag.String("addr", "localhost:7401", "rstmdb server address")
	token := flag.String("token", "my-secret-token", "bearer auth token (optional)")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := rstmdb.Connect(ctx, *addr, &rstmdb.Options{
		Auth:       *token,
		ClientName: "example-wal",
	})
	if err != nil {
		log.Fatalf("Connect: %v", err)
	}
	defer client.Close()
	fmt.Println("Connected to", *addr)

	// --- Set up some data to populate the WAL ---
	_, err = client.PutMachine(ctx, rstmdb.PutMachineRequest{
		Machine: "counter",
		Version: 1,
		Definition: rstmdb.MachineDefinition{
			States:  []string{"idle", "counting"},
			Initial: "idle",
			Transitions: []rstmdb.Transition{
				{From: rstmdb.StringOrSlice{"idle"}, Event: "START", To: "counting"},
				{From: rstmdb.StringOrSlice{"counting"}, Event: "RESET", To: "idle"},
			},
		},
	})
	if err != nil {
		log.Fatalf("PutMachine: %v", err)
	}

	inst, err := client.CreateInstance(ctx, rstmdb.CreateInstanceRequest{
		InstanceID: "counter-1",
		Machine:    "counter",
		Version:    1,
	})
	if err != nil {
		log.Fatalf("CreateInstance: %v", err)
	}
	fmt.Printf("Created instance: %s (wal_offset=%d)\n", inst.InstanceID, inst.WALOffset)

	_, err = client.ApplyEvent(ctx, rstmdb.ApplyEventRequest{
		InstanceID: "counter-1",
		Event:      "START",
	})
	if err != nil {
		log.Fatalf("ApplyEvent: %v", err)
	}
	fmt.Println("Applied START event")

	// --- WAL Stats ---
	fmt.Println("\n--- WAL Stats ---")
	stats, err := client.WALStats(ctx)
	if err != nil {
		log.Fatalf("WALStats: %v", err)
	}
	fmt.Printf("  Entries:        %d\n", stats.EntryCount)
	fmt.Printf("  Segments:       %d\n", stats.SegmentCount)
	fmt.Printf("  Total size:     %d bytes\n", stats.TotalSize)
	fmt.Printf("  Latest offset:  %d\n", stats.LatestOffset)
	fmt.Printf("  IO Stats:\n")
	fmt.Printf("    Bytes written: %d\n", stats.IOStats.BytesWritten)
	fmt.Printf("    Bytes read:    %d\n", stats.IOStats.BytesRead)
	fmt.Printf("    Writes:        %d\n", stats.IOStats.Writes)
	fmt.Printf("    Reads:         %d\n", stats.IOStats.Reads)
	fmt.Printf("    Fsyncs:        %d\n", stats.IOStats.Fsyncs)

	// --- WAL Read ---
	fmt.Println("\n--- WAL Read (from offset 0, limit 10) ---")
	walResult, err := client.WALRead(ctx, 0, rstmdb.WALLimit(10))
	if err != nil {
		log.Fatalf("WALRead: %v", err)
	}
	fmt.Printf("  Records: %d, next_offset: %d\n", len(walResult.Records), walResult.NextOffset)
	for i, rec := range walResult.Records {
		fmt.Printf("  [%d] seq=%d offset=%d entry=%v\n", i, rec.Sequence, rec.Offset, rec.Entry)
	}

	// --- Snapshot ---
	fmt.Println("\n--- Snapshot Instance ---")
	snap, err := client.SnapshotInstance(ctx, "counter-1")
	if err != nil {
		log.Fatalf("SnapshotInstance: %v", err)
	}
	fmt.Printf("  Instance:  %s\n", snap.InstanceID)
	fmt.Printf("  Snapshot:  %s\n", snap.SnapshotID)
	fmt.Printf("  WAL offset: %d\n", snap.WALOffset)
	if snap.SizeBytes > 0 {
		fmt.Printf("  Size:      %d bytes\n", snap.SizeBytes)
	}
	if snap.Checksum != "" {
		fmt.Printf("  Checksum:  %s\n", snap.Checksum)
	}

	// --- Compact ---
	fmt.Println("\n--- Compact (force_snapshot=false) ---")
	compact, err := client.Compact(ctx, false)
	if err != nil {
		log.Fatalf("Compact: %v", err)
	}
	fmt.Printf("  Snapshots created: %d\n", compact.SnapshotsCreated)
	fmt.Printf("  Segments deleted:  %d\n", compact.SegmentsDeleted)
	fmt.Printf("  Bytes reclaimed:   %d\n", compact.BytesReclaimed)
	fmt.Printf("  Total snapshots:   %d\n", compact.TotalSnapshots)
	fmt.Printf("  WAL segments:      %d\n", compact.WALSegments)

	fmt.Println("\nDone!")
}
