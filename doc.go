// Package rstmdb provides a Go client for the rstmdb state machine database.
//
// Basic usage:
//
//	client, err := rstmdb.Connect(ctx, "localhost:7401", &rstmdb.Options{
//	    Auth:       "my-token",
//	    ClientName: "my-service",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close()
//
//	// Register a machine
//	_, err = client.PutMachine(ctx, rstmdb.PutMachineRequest{
//	    Machine: "order",
//	    Version: 1,
//	    Definition: rstmdb.MachineDefinition{
//	        States:  []string{"created", "paid", "shipped"},
//	        Initial: "created",
//	        Transitions: []rstmdb.Transition{
//	            {From: rstmdb.StringOrSlice{"created"}, Event: "PAY", To: "paid"},
//	            {From: rstmdb.StringOrSlice{"paid"}, Event: "SHIP", To: "shipped"},
//	        },
//	    },
//	})
//
//	// Create an instance
//	inst, err := client.CreateInstance(ctx, rstmdb.CreateInstanceRequest{
//	    Machine:    "order",
//	    Version:    1,
//	    InstanceID: "order-123",
//	    InitialCtx: map[string]any{"customer": "alice"},
//	})
//
//	// Apply an event
//	result, err := client.ApplyEvent(ctx, rstmdb.ApplyEventRequest{
//	    InstanceID: "order-123",
//	    Event:      "PAY",
//	    Payload:    map[string]any{"amount": 99.99},
//	})
//
// For streaming events:
//
//	sub, err := client.WatchAll(ctx,
//	    rstmdb.WatchMachines("order"),
//	    rstmdb.WatchToStates("shipped"),
//	)
//	defer sub.Close()
//
//	for event := range sub.Events {
//	    fmt.Printf("Order %s shipped!\n", event.InstanceID)
//	}
package rstmdb
