// Example: streaming events with the rstmdb Go client.
//
// Subscribes to all events and prints them forever until interrupted (Ctrl+C).
// Optionally filter by machine, event name, or target state.
//
// Usage:
//
//	go run ./examples/streaming -addr localhost:7401 [-token my-secret]
//	go run ./examples/streaming -addr localhost:7401 -machine order
//	go run ./examples/streaming -addr localhost:7401 -machine order -to-state shipped,delivered
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	rstmdb "github.com/rstmdb/rstmdb-go"
)

func main() {
	addr := flag.String("addr", "localhost:7401", "rstmdb server address")
	token := flag.String("token", "", "bearer auth token")
	machine := flag.String("machine", "", "filter by machine name (comma-separated)")
	toState := flag.String("to-state", "", "filter by target state (comma-separated)")
	includeCtx := flag.Bool("ctx", true, "include instance context in events")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	client, err := rstmdb.Connect(ctx, *addr, &rstmdb.Options{
		Auth:       *token,
		ClientName: "example-streaming",
	})
	if err != nil {
		log.Fatalf("Connect: %v", err)
	}
	defer client.Close()

	// Build watch options from flags
	var opts []rstmdb.WatchOption
	opts = append(opts, rstmdb.WatchIncludeCtx(*includeCtx))
	if *machine != "" {
		opts = append(opts, rstmdb.WatchMachines(strings.Split(*machine, ",")...))
	}
	if *toState != "" {
		opts = append(opts, rstmdb.WatchToStates(strings.Split(*toState, ",")...))
	}

	sub, err := client.WatchAll(ctx, opts...)
	if err != nil {
		log.Fatalf("WatchAll: %v", err)
	}
	defer sub.Close()

	fmt.Fprintf(os.Stderr, "Listening for events on %s (subscription=%s) — Ctrl+C to stop\n", *addr, sub.ID)

	count := 0
	for {
		select {
		case event := <-sub.Events:
			count++
			fmt.Printf("[%d] %s %s: %s -> %s  (event=%s, wal_offset=%d)\n",
				count, event.Machine, event.InstanceID,
				event.FromState, event.ToState,
				event.EventName, event.WALOffset)
			if *includeCtx && event.Ctx != nil {
				fmt.Printf("     ctx=%v\n", event.Ctx)
			}
		case err := <-sub.Errors:
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		case <-ctx.Done():
			fmt.Fprintf(os.Stderr, "\nReceived %d events. Bye!\n", count)
			return
		}
	}
}
