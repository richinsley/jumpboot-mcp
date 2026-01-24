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
	"github.com/richinsley/jumpboot-mcp/internal/discovery"
	"github.com/richinsley/jumpboot-mcp/internal/manager"
	"github.com/richinsley/jumpboot-mcp/internal/proxy"
	mcpserver "github.com/richinsley/jumpboot-mcp/internal/server"
)

func main() {
	// Transport flags
	transport := flag.String("transport", "stdio", "Transport type: stdio, http")
	addr := flag.String("addr", ":8080", "HTTP server address (for http transport)")
	endpoint := flag.String("endpoint", "/mcp", "HTTP endpoint path (for http transport)")
	stateless := flag.Bool("stateless", false, "Run HTTP server in stateless mode")
	certFile := flag.String("tls-cert", "", "TLS certificate file (enables HTTPS)")
	keyFile := flag.String("tls-key", "", "TLS key file (enables HTTPS)")

	// mDNS flags
	note := flag.String("note", "", "Human-readable server description (e.g., 'GPU server for ML')")
	instanceName := flag.String("instance-name", discovery.GetDefaultInstanceName(), "Unique mDNS instance name")
	mdnsAnnounce := flag.Bool("mdns-announce", true, "Enable mDNS service announcement (HTTP mode)")
	mdnsDiscover := flag.Bool("mdns-discover", true, "Enable mDNS service discovery (stdio mode)")
	discoverTimeout := flag.Duration("discover-timeout", 5*time.Second, "Discovery wait time at startup")

	flag.Parse()

	// Create the environment manager
	mgr, err := manager.NewManager("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create manager: %v\n", err)
		os.Exit(1)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	switch *transport {
	case "stdio":
		runStdioMode(mgr, sigChan, *mdnsDiscover, *discoverTimeout)

	case "http":
		runHTTPMode(mgr, sigChan, *addr, *endpoint, *stateless, *certFile, *keyFile,
			*note, *instanceName, *mdnsAnnounce)

	default:
		fmt.Fprintf(os.Stderr, "Unknown transport: %s (use 'stdio' or 'http')\n", *transport)
		os.Exit(1)
	}
}

func runStdioMode(mgr *manager.Manager, sigChan chan os.Signal, discover bool, discoverTimeout time.Duration) {
	var aggregator *proxy.ToolAggregator

	// Discover remote services if enabled
	if discover {
		aggregator = proxy.NewToolAggregator()
		ctx, cancel := context.WithTimeout(context.Background(), discoverTimeout)
		services, err := discovery.Discover(ctx, discoverTimeout)
		cancel()

		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: mDNS discovery failed: %v\n", err)
		} else if len(services) > 0 {
			fmt.Fprintf(os.Stderr, "Discovered %d remote jumpboot-mcp service(s):\n", len(services))
			for _, svc := range services {
				fmt.Fprintf(os.Stderr, "  - %s at %s", svc.InstanceName, svc.URL())
				if svc.Note != "" {
					fmt.Fprintf(os.Stderr, " (%s)", svc.Note)
				}
				fmt.Fprintln(os.Stderr)

				// Connect to the remote service
				if err := aggregator.AddRemote(context.Background(), svc); err != nil {
					fmt.Fprintf(os.Stderr, "    Warning: failed to connect: %v\n", err)
				} else {
					fmt.Fprintf(os.Stderr, "    Connected successfully\n")
				}
			}
		}
	}

	// Create the MCP server with local tools + proxy tools
	var s *server.MCPServer
	if aggregator != nil && aggregator.RemoteCount() > 0 {
		proxyTools := aggregator.GetAllTools()
		fmt.Fprintf(os.Stderr, "Registered %d proxied tools from remote servers\n", len(proxyTools))
		s = mcpserver.NewWithExtraTools(mgr, proxyTools)
	} else {
		s = mcpserver.New(mgr)
	}

	// Handle shutdown
	go func() {
		<-sigChan
		if aggregator != nil {
			aggregator.Close()
		}
		mgr.Shutdown()
		os.Exit(0)
	}()

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func runHTTPMode(mgr *manager.Manager, sigChan chan os.Signal, addr, endpoint string,
	stateless bool, certFile, keyFile, note, instanceName string, announce bool) {

	// Create the MCP server
	s := mcpserver.New(mgr)

	// Build HTTP server options
	opts := []server.StreamableHTTPOption{
		server.WithEndpointPath(endpoint),
		server.WithHeartbeatInterval(30 * time.Second),
	}

	if stateless {
		opts = append(opts, server.WithStateLess(true))
	}

	useTLS := certFile != "" && keyFile != ""
	if useTLS {
		opts = append(opts, server.WithTLSCert(certFile, keyFile))
	}

	// Create the HTTP server
	httpServer := server.NewStreamableHTTPServer(s, opts...)

	// Start mDNS announcer if enabled
	var announcer *discovery.Announcer
	if announce {
		port, err := discovery.ParsePort(addr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse port from %s: %v\n", addr, err)
		} else {
			info := discovery.ServiceInfo{
				InstanceName: instanceName,
				Port:         port,
				Note:         note,
				Endpoint:     endpoint,
				TLS:          useTLS,
			}

			announcer = discovery.NewAnnouncer(info)
			if err := announcer.Start(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to start mDNS announcer: %v\n", err)
				announcer = nil
			} else {
				fmt.Fprintf(os.Stderr, "mDNS: announcing as '%s' on port %d\n", instanceName, port)
				if note != "" {
					fmt.Fprintf(os.Stderr, "mDNS: note = %s\n", note)
				}
			}
		}
	}

	// Handle graceful shutdown
	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "Shutting down...")

		// Stop mDNS announcer
		if announcer != nil {
			announcer.Stop()
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		httpServer.Shutdown(ctx)
		mgr.Shutdown()
		os.Exit(0)
	}()

	proto := "http"
	if useTLS {
		proto = "https"
	}
	fmt.Fprintf(os.Stderr, "Starting MCP server on %s://%s%s\n", proto, addr, endpoint)

	if err := httpServer.Start(addr); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
