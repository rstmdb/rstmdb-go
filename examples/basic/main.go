// Example: basic usage of the rstmdb Go client.
//
// Demonstrates connecting, registering a machine, creating an instance,
// applying events, querying state, and cleaning up.
//
// Usage:
//
//	go run ./examples/basic -addr localhost:7401 [-token my-secret]
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
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
		Auth:           *token,
		ClientName:     "example-basic",
		Timeout:        5 * time.Second,
		RequestTimeout: 10 * time.Second,
	})
	if err != nil {
		log.Fatalf("Connect: %v", err)
	}
	defer client.Close()
	fmt.Println("Connected to", *addr)

	// --- Ping ---
	if err := client.Ping(ctx); err != nil {
		log.Fatalf("Ping: %v", err)
	}
	fmt.Println("Ping OK")

	// --- Server Info ---
	info, err := client.Info(ctx)
	if err != nil {
		log.Fatalf("Info: %v", err)
	}
	fmt.Printf("Server: %s %s (protocol v%d)\n",
		info.ServerName, info.ServerVersion, info.ProtocolVersion)
	fmt.Printf("Features: %v\n", info.Features)

	// --- Register a machine ---
	putResult, err := client.PutMachine(ctx, rstmdb.PutMachineRequest{
		Machine: "order-go-example-v1",
		Version: 1,
		Definition: rstmdb.MachineDefinition{
			States:  []string{"created", "paid", "shipped", "delivered", "cancelled"},
			Initial: "created",
			Transitions: []rstmdb.Transition{
				{From: rstmdb.StringOrSlice{"created"}, Event: "PAY", To: "paid"},
				{From: rstmdb.StringOrSlice{"paid"}, Event: "SHIP", To: "shipped"},
				{From: rstmdb.StringOrSlice{"shipped"}, Event: "DELIVER", To: "delivered"},
				{From: rstmdb.StringOrSlice{"created", "paid"}, Event: "CANCEL", To: "cancelled"},
			},
		},
	})
	if err != nil {
		log.Fatalf("PutMachine: %v", err)
	}
	fmt.Printf("Machine registered: %s v%d (created=%v, checksum=%s)\n",
		putResult.Machine, putResult.Version, putResult.Created, putResult.StoredChecksum)

	// --- List machines ---
	machines, err := client.ListMachines(ctx)
	if err != nil {
		log.Fatalf("ListMachines: %v", err)
	}
	fmt.Printf("Machines (%d):\n", len(machines))
	for _, m := range machines {
		fmt.Printf("  %s versions=%v\n", m.Machine, m.Versions)
	}

	// --- Get machine definition ---
	machineInfo, err := client.GetMachine(ctx, "order-go-example-v1", 1)
	if err != nil {
		log.Fatalf("GetMachine: %v", err)
	}
	fmt.Printf("Machine 'order' has %d states, initial=%q\n",
		len(machineInfo.Definition.States), machineInfo.Definition.Initial)

	// --- Create an instance ---
	inst, err := client.CreateInstance(ctx, rstmdb.CreateInstanceRequest{
		InstanceID: "order-001",
		Machine:    "order-go-example-v1",
		Version:    1,
		InitialCtx: map[string]any{
			"customer": "alice",
			"items":    []string{"widget-a", "widget-b"},
			"total":    49.99,
		},
	})
	if err != nil {
		log.Fatalf("CreateInstance: %v", err)
	}
	fmt.Printf("Instance created: %s (state=%s, wal_offset=%d)\n",
		inst.InstanceID, inst.State, inst.WALOffset)

	// --- Apply events to drive state transitions ---
	transitions := []struct {
		event   string
		payload map[string]any
	}{
		{"PAY", map[string]any{"method": "credit_card", "amount": 49.99}},
		{"SHIP", map[string]any{"carrier": "fedex", "tracking": "FX123456"}},
		{"DELIVER", map[string]any{"signed_by": "Alice"}},
	}

	for _, tr := range transitions {
		result, err := client.ApplyEvent(ctx, rstmdb.ApplyEventRequest{
			InstanceID: "order-001",
			Event:      tr.event,
			Payload:    tr.payload,
		})
		if err != nil {
			if rstmdb.IsInvalidTransition(err) {
				fmt.Printf("  %s: invalid transition (expected)\n", tr.event)
				continue
			}
			log.Fatalf("ApplyEvent(%s): %v", tr.event, err)
		}
		fmt.Printf("  %s: %s -> %s (applied=%v, wal_offset=%d)\n",
			tr.event, result.FromState, result.ToState, result.Applied, result.WALOffset)
	}

	// --- Get final instance state ---
	instance, err := client.GetInstance(ctx, "order-001")
	if err != nil {
		log.Fatalf("GetInstance: %v", err)
	}
	fmt.Printf("Final state: %s (machine=%s v%d)\n",
		instance.State, instance.Machine, instance.Version)
	fmt.Printf("Context: %v\n", instance.Ctx)

	// --- List instances ---
	list, err := client.ListInstances(ctx,
		rstmdb.WithMachine("order-go-example-v1"),
		rstmdb.WithLimit(10),
	)
	if err != nil {
		log.Fatalf("ListInstances: %v", err)
	}
	fmt.Printf("Instances (total=%d, has_more=%v):\n", list.Total, list.HasMore)
	for _, i := range list.Instances {
		fmt.Printf("  %s: machine=%s state=%s\n", i.ID, i.Machine, i.State)
	}

	// --- Delete instance ---
	delResult, err := client.DeleteInstance(ctx, "order-001")
	if err != nil {
		log.Fatalf("DeleteInstance: %v", err)
	}
	fmt.Printf("Deleted: %s (deleted=%v)\n", delResult.InstanceID, delResult.Deleted)

	// --- Demonstrate error handling ---
	_, err = client.GetInstance(ctx, "nonexistent-id")
	if err != nil {
		if rstmdb.IsInstanceNotFound(err) {
			fmt.Println("GetInstance nonexistent: correctly got INSTANCE_NOT_FOUND")
		} else if rstmdb.IsNotFound(err) {
			fmt.Println("GetInstance nonexistent: correctly got NOT_FOUND")
		} else {
			fmt.Printf("GetInstance nonexistent: %v\n", err)
		}
	}

	fmt.Println("\nDone!")
	os.Exit(0)
}
