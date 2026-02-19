// Example: batch operations with the rstmdb Go client.
//
// Demonstrates atomic and best-effort batch modes, batch helpers,
// and result inspection.
//
// Usage:
//
//	go run ./examples/batch -addr localhost:7401 [-token my-secret]
package main

import (
	"context"
	"encoding/json"
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
		ClientName: "example-batch",
	})
	if err != nil {
		log.Fatalf("Connect: %v", err)
	}
	defer client.Close()
	fmt.Println("Connected to", *addr)

	// --- Set up machine ---
	_, err = client.PutMachine(ctx, rstmdb.PutMachineRequest{
		Machine: "task",
		Version: 1,
		Definition: rstmdb.MachineDefinition{
			States:  []string{"open", "in_progress", "done", "archived"},
			Initial: "open",
			Transitions: []rstmdb.Transition{
				{From: rstmdb.StringOrSlice{"open"}, Event: "START", To: "in_progress"},
				{From: rstmdb.StringOrSlice{"in_progress"}, Event: "COMPLETE", To: "done"},
				{From: rstmdb.StringOrSlice{"done"}, Event: "ARCHIVE", To: "archived"},
			},
		},
	})
	if err != nil {
		log.Fatalf("PutMachine: %v", err)
	}
	fmt.Println("Machine 'task' registered")

	// --- Atomic batch: create multiple instances at once ---
	fmt.Println("\n--- Atomic Batch: Create Instances ---")
	createOps := []rstmdb.BatchOperation{
		rstmdb.BatchCreateInstance(rstmdb.CreateInstanceRequest{
			InstanceID: "task-1",
			Machine:    "task",
			Version:    1,
			InitialCtx: map[string]any{"title": "Design API", "assignee": "alice"},
		}),
		rstmdb.BatchCreateInstance(rstmdb.CreateInstanceRequest{
			InstanceID: "task-2",
			Machine:    "task",
			Version:    1,
			InitialCtx: map[string]any{"title": "Write tests", "assignee": "bob"},
		}),
		rstmdb.BatchCreateInstance(rstmdb.CreateInstanceRequest{
			InstanceID: "task-3",
			Machine:    "task",
			Version:    1,
			InitialCtx: map[string]any{"title": "Deploy", "assignee": "charlie"},
		}),
	}

	results, err := client.Batch(ctx, rstmdb.BatchAtomic, createOps)
	if err != nil {
		log.Fatalf("Batch create: %v", err)
	}

	fmt.Printf("Created %d instances:\n", len(results))
	for i, r := range results {
		fmt.Printf("  [%d] status=%s result=%s\n", i, r.Status, string(r.Result))
	}

	// --- Best-effort batch: apply events to multiple instances ---
	fmt.Println("\n--- Best-Effort Batch: Apply Events ---")
	eventOps := []rstmdb.BatchOperation{
		rstmdb.BatchApplyEvent(rstmdb.ApplyEventRequest{
			InstanceID: "task-1",
			Event:      "START",
		}),
		rstmdb.BatchApplyEvent(rstmdb.ApplyEventRequest{
			InstanceID: "task-2",
			Event:      "START",
		}),
		rstmdb.BatchApplyEvent(rstmdb.ApplyEventRequest{
			InstanceID: "task-1",
			Event:      "COMPLETE",
		}),
	}

	results, err = client.Batch(ctx, rstmdb.BatchBestEffort, eventOps)
	if err != nil {
		log.Fatalf("Batch events: %v", err)
	}

	fmt.Printf("Applied %d events:\n", len(results))
	for i, r := range results {
		if r.Status == "ok" {
			fmt.Printf("  [%d] OK: %s\n", i, string(r.Result))
		} else {
			fmt.Printf("  [%d] ERROR: %v\n", i, r.Error)
		}
	}

	// --- Mixed batch: different operation types ---
	fmt.Println("\n--- Mixed Batch ---")
	mixedOps := []rstmdb.BatchOperation{
		rstmdb.BatchApplyEvent(rstmdb.ApplyEventRequest{
			InstanceID: "task-1",
			Event:      "ARCHIVE",
		}),
		rstmdb.BatchDeleteInstance("task-3"),
	}

	results, err = client.Batch(ctx, rstmdb.BatchBestEffort, mixedOps)
	if err != nil {
		log.Fatalf("Batch mixed: %v", err)
	}

	fmt.Printf("Mixed batch (%d results):\n", len(results))
	for i, r := range results {
		if r.Status == "ok" {
			fmt.Printf("  [%d] OK: %s\n", i, string(r.Result))
		} else {
			fmt.Printf("  [%d] ERROR: %v\n", i, r.Error)
		}
	}

	// --- Custom BatchOperation ---
	fmt.Println("\n--- Custom BatchOperation ---")
	customOps := []rstmdb.BatchOperation{
		{
			Op: "GET_INSTANCE",
			Params: map[string]any{
				"instance_id": "task-2",
			},
		},
	}

	results, err = client.Batch(ctx, rstmdb.BatchBestEffort, customOps)
	if err != nil {
		log.Fatalf("Batch custom: %v", err)
	}
	for _, r := range results {
		if r.Status == "ok" {
			var inst map[string]any
			json.Unmarshal(r.Result, &inst)
			fmt.Printf("  task-2 state: %v\n", inst["state"])
		}
	}

	fmt.Println("\nDone!")
}
