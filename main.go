package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"github.com/richinsley/jumpboot-mcp/internal/manager"
	mcpserver "github.com/richinsley/jumpboot-mcp/internal/server"
)

func main() {
	// Command line flags
	transport := flag.String("transport", "stdio", "Transport type: stdio, http")
	addr := flag.String("addr", ":8080", "HTTP server address (for http transport)")
	endpoint := flag.String("endpoint", "/mcp", "HTTP endpoint path (for http transport)")
	stateless := flag.Bool("stateless", false, "Run HTTP server in stateless mode")
	certFile := flag.String("tls-cert", "", "TLS certificate file (enables HTTPS)")
	keyFile := flag.String("tls-key", "", "TLS key file (enables HTTPS)")
	flag.Parse()

	// Create the environment manager
	mgr, err := manager.NewManager("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create manager: %v\n", err)
		os.Exit(1)
	}

	// Create the MCP server
	s := mcpserver.New(mgr)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	switch *transport {
	case "stdio":
		go func() {
			<-sigChan
			mgr.Shutdown()
			os.Exit(0)
		}()

		if err := server.ServeStdio(s); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}

	case "http":
		// Build HTTP server options
		opts := []server.StreamableHTTPOption{
			server.WithEndpointPath(*endpoint),
			server.WithHeartbeatInterval(30 * time.Second),
		}

		if *stateless {
			opts = append(opts, server.WithStateLess(true))
		}

		if *certFile != "" && *keyFile != "" {
			opts = append(opts, server.WithTLSCert(*certFile, *keyFile))
		}

		// Create the HTTP server
		httpServer := server.NewStreamableHTTPServer(s, opts...)

		// Handle graceful shutdown
		go func() {
			<-sigChan
			fmt.Fprintln(os.Stderr, "Shutting down...")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			httpServer.Shutdown(ctx)
			mgr.Shutdown()
			os.Exit(0)
		}()

		proto := "http"
		if *certFile != "" {
			proto = "https"
		}
		fmt.Fprintf(os.Stderr, "Starting MCP server on %s://%s%s\n", proto, *addr, *endpoint)

		if err := httpServer.Start(*addr); err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}

	default:
		fmt.Fprintf(os.Stderr, "Unknown transport: %s (use 'stdio' or 'http')\n", *transport)
		os.Exit(1)
	}
}
