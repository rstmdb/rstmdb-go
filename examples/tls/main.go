// Example: TLS/mTLS connections with the rstmdb Go client.
//
// Shows three TLS configurations:
//   - Custom TLS from PEM files (mTLS)
//   - Insecure TLS (skip verification, dev only)
//   - Custom *tls.Config
//
// Usage:
//
//	go run ./examples/tls -addr secure.example.com:7401 \
//	    -ca ca.pem -cert client.pem -key client-key.pem \
//	    [-token my-secret]
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"time"

	rstmdb "github.com/rstmdb/rstmdb-go"
)

func main() {
	addr := flag.String("addr", "localhost:7401", "rstmdb server address")
	token := flag.String("token", "", "bearer auth token")
	caFile := flag.String("ca", "", "CA certificate PEM file")
	certFile := flag.String("cert", "", "client certificate PEM file")
	keyFile := flag.String("key", "", "client private key PEM file")
	insecure := flag.Bool("insecure", false, "skip TLS certificate verification (dev only)")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := &rstmdb.Options{
		Auth:       *token,
		ClientName: "example-tls",
	}

	// Choose TLS configuration method
	switch {
	case *insecure:
		fmt.Println("Using insecure TLS (skip verification)")
		opts.TLS = rstmdb.InsecureTLS()

	case *caFile != "" || *certFile != "":
		fmt.Printf("Using TLS from files: ca=%s cert=%s key=%s\n", *caFile, *certFile, *keyFile)
		tlsCfg, err := rstmdb.TLSFromFiles(*caFile, *certFile, *keyFile)
		if err != nil {
			log.Fatalf("TLSFromFiles: %v", err)
		}
		opts.TLS = tlsCfg

	default:
		fmt.Println("Using custom tls.Config with system root CAs")
		opts.TLS = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	client, err := rstmdb.Connect(ctx, *addr, opts)
	if err != nil {
		log.Fatalf("Connect: %v", err)
	}
	defer client.Close()

	fmt.Println("Connected with TLS to", *addr)

	if err := client.Ping(ctx); err != nil {
		log.Fatalf("Ping: %v", err)
	}
	fmt.Println("Ping OK")

	info, err := client.Info(ctx)
	if err != nil {
		log.Fatalf("Info: %v", err)
	}
	fmt.Printf("Server: %s %s (protocol v%d)\n",
		info.ServerName, info.ServerVersion, info.ProtocolVersion)

	fmt.Println("\nTLS connection successful!")
}
