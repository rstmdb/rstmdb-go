# rstmdb-go

Official Go client for [rstmdb](https://github.com/rstmdb/rstmdb) — the state machine database.

Implements the full RCP protocol: RCPX binary framing, CRC32C checksums, all 22 operations, 16 error codes, request/response multiplexing, and async subscription streaming on a single TCP connection.

**Zero external dependencies.** Standard library only.

## Install

```
go get github.com/rstmdb/rstmdb-go
```

Requires Go 1.21+.

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    rstmdb "github.com/rstmdb/rstmdb-go"
)

func main() {
    ctx := context.Background()

    client, err := rstmdb.Connect(ctx, "localhost:7401", &rstmdb.Options{
        Auth:       "my-token",
        ClientName: "my-service",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Register a state machine
    _, err = client.PutMachine(ctx, rstmdb.PutMachineRequest{
        Machine: "order",
        Version: 1,
        Definition: rstmdb.MachineDefinition{
            States:  []string{"created", "paid", "shipped"},
            Initial: "created",
            Transitions: []rstmdb.Transition{
                {From: rstmdb.StringOrSlice{"created"}, Event: "PAY", To: "paid"},
                {From: rstmdb.StringOrSlice{"paid"}, Event: "SHIP", To: "shipped"},
            },
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    // Create an instance
    inst, err := client.CreateInstance(ctx, rstmdb.CreateInstanceRequest{
        Machine:    "order",
        Version:    1,
        InstanceID: "order-123",
        InitialCtx: map[string]any{"customer": "alice"},
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println("State:", inst.State) // "created"

    // Apply an event
    result, err := client.ApplyEvent(ctx, rstmdb.ApplyEventRequest{
        InstanceID: "order-123",
        Event:      "PAY",
        Payload:    map[string]any{"amount": 99.99},
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("%s -> %s\n", result.FromState, result.ToState) // "created -> paid"
}
```

## Connect

Pass `nil` for defaults, or set only the fields you need:

```go
client, err := rstmdb.Connect(ctx, "localhost:7401", &rstmdb.Options{
    Auth:           "bearer-token",
    ClientName:     "my-service",
    Timeout:        5 * time.Second,
    RequestTimeout: 10 * time.Second,
    WireMode:       "binary_json", // or "jsonl"
    Features:       []string{"watch", "batch"},
})
```

### TLS

```go
// From PEM files (mTLS)
tlsCfg, err := rstmdb.TLSFromFiles("ca.pem", "client.pem", "client-key.pem")
client, err := rstmdb.Connect(ctx, addr, &rstmdb.Options{TLS: tlsCfg})

// Insecure (dev only)
client, err := rstmdb.Connect(ctx, addr, &rstmdb.Options{TLS: rstmdb.InsecureTLS()})

// Custom *tls.Config
client, err := rstmdb.Connect(ctx, addr, &rstmdb.Options{TLS: myTLSConfig})
```

## Operations

### Machine Operations

```go
result, err := client.PutMachine(ctx, req)       // Register/update a machine
info, err := client.GetMachine(ctx, "name", 1)   // Get machine definition
machines, err := client.ListMachines(ctx)         // List all machines
```

### Instance Operations

```go
result, err := client.CreateInstance(ctx, req)    // Create instance
inst, err := client.GetInstance(ctx, "id")        // Get instance
list, err := client.ListInstances(ctx,            // List with filters
    rstmdb.WithMachine("order"),
    rstmdb.WithState("paid"),
    rstmdb.WithLimit(50),
)
del, err := client.DeleteInstance(ctx, "id")      // Delete instance
```

### Events

```go
result, err := client.ApplyEvent(ctx, rstmdb.ApplyEventRequest{
    InstanceID:    "order-123",
    Event:         "PAY",
    Payload:       map[string]any{"amount": 99.99},
    ExpectedState: "created", // optimistic concurrency
})
```

### Batch

```go
results, err := client.Batch(ctx, rstmdb.BatchAtomic, []rstmdb.BatchOperation{
    rstmdb.BatchCreateInstance(rstmdb.CreateInstanceRequest{...}),
    rstmdb.BatchApplyEvent(rstmdb.ApplyEventRequest{...}),
    rstmdb.BatchDeleteInstance("instance-id"),
})
```

### Streaming

```go
sub, err := client.WatchAll(ctx,
    rstmdb.WatchMachines("order"),
    rstmdb.WatchToStates("shipped"),
)
defer sub.Close()

for {
    select {
    case event := <-sub.Events:
        fmt.Printf("%s: %s -> %s\n", event.InstanceID, event.FromState, event.ToState)
    case err := <-sub.Errors:
        log.Println("watch error:", err)
    case <-ctx.Done():
        return
    }
}
```

Or watch a single instance:

```go
sub, err := client.WatchInstance(ctx, rstmdb.WatchInstanceRequest{
    InstanceID: "order-123",
    IncludeCtx: true,
})
```

### WAL & Snapshots

```go
stats, err := client.WALStats(ctx)
records, err := client.WALRead(ctx, 0, rstmdb.WALLimit(100))
snap, err := client.SnapshotInstance(ctx, "instance-id")
compact, err := client.Compact(ctx, false)
```

### System

```go
err := client.Ping(ctx)
info, err := client.Info(ctx)
client.Close()
```

## Error Handling

```go
result, err := client.ApplyEvent(ctx, req)
if err != nil {
    var e *rstmdb.Error
    if errors.As(err, &e) {
        fmt.Println(e.Code, e.Message, e.Retryable)
    }

    // Convenience helpers
    if rstmdb.IsInstanceNotFound(err)   { /* ... */ }
    if rstmdb.IsInvalidTransition(err)  { /* ... */ }
    if rstmdb.IsConflict(err)           { /* ... */ }
    if rstmdb.IsRetryable(err)          { /* retry */ }
}
```

All 16 error codes are available as `rstmdb.ErrCode*` constants.

## Examples

```
go run ./examples/basic     -addr localhost:7401   # Full workflow
go run ./examples/streaming -addr localhost:7401   # Live event tail
go run ./examples/batch     -addr localhost:7401   # Batch operations
go run ./examples/tls       -addr host:7401 -ca ca.pem -cert c.pem -key k.pem
go run ./examples/wal       -addr localhost:7401   # WAL/snapshot ops
```

## Development

```bash
make              # fmt + vet + build + test -race
make test         # Run tests
make test-verbose # Run tests with -v
make test-cover   # Coverage report
make lint         # staticcheck (or go vet)
make examples     # Build all examples
make clean        # Remove artifacts
```

## License

[MIT](LICENSE)
