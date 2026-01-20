package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mark3labs/mcp-go/server"
	"github.com/richinsley/jumpboot-mcp/internal/manager"
	mcpserver "github.com/richinsley/jumpboot-mcp/internal/server"
)

func main() {
	// Create the environment manager
	mgr, err := manager.NewManager("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create manager: %v\n", err)
		os.Exit(1)
	}

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		mgr.Shutdown()
		os.Exit(0)
	}()

	// Create and run the MCP server
	s := mcpserver.New(mgr)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
