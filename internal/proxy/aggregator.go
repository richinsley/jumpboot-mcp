package proxy

import (
	"context"
	"fmt"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/richinsley/jumpboot-mcp/internal/discovery"
	"github.com/richinsley/jumpboot-mcp/internal/tools"
)

// toolSource tracks the origin of a prefixed tool
type toolSource struct {
	remote       *RemoteClient
	originalName string
}

// ToolAggregator aggregates tools from multiple remote MCP servers
type ToolAggregator struct {
	remotes     map[string]*RemoteClient // instance name -> client
	toolMapping map[string]toolSource    // prefixed tool name -> source
	mu          sync.RWMutex
}

// NewToolAggregator creates a new tool aggregator
func NewToolAggregator() *ToolAggregator {
	return &ToolAggregator{
		remotes:     make(map[string]*RemoteClient),
		toolMapping: make(map[string]toolSource),
	}
}

// AddRemote connects to a remote service and adds its tools
func (a *ToolAggregator) AddRemote(ctx context.Context, info discovery.ServiceInfo) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Check if already connected
	if _, exists := a.remotes[info.InstanceName]; exists {
		return nil
	}

	// Create and connect to the remote
	remote := NewRemoteClient(info)
	if err := remote.Connect(ctx); err != nil {
		return err
	}

	// Add to remotes map
	a.remotes[info.InstanceName] = remote

	// Add tool mappings
	for _, tool := range remote.Tools() {
		prefixedName := fmt.Sprintf("%s:%s", info.InstanceName, tool.Name)
		a.toolMapping[prefixedName] = toolSource{
			remote:       remote,
			originalName: tool.Name,
		}
	}

	return nil
}

// RemoveRemote disconnects from a remote service
func (a *ToolAggregator) RemoveRemote(instanceName string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	remote, exists := a.remotes[instanceName]
	if !exists {
		return nil
	}

	// Remove tool mappings for this remote
	for name, source := range a.toolMapping {
		if source.remote == remote {
			delete(a.toolMapping, name)
		}
	}

	// Close and remove the remote
	delete(a.remotes, instanceName)
	return remote.Close()
}

// GetAllTools returns tool definitions for all remote tools with prefixed names
func (a *ToolAggregator) GetAllTools() []tools.ToolDef {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []tools.ToolDef

	for instanceName, remote := range a.remotes {
		for _, tool := range remote.Tools() {
			prefixedName := fmt.Sprintf("%s:%s", instanceName, tool.Name)

			// Create enhanced description with note
			description := tool.Description
			if remote.Info.Note != "" {
				description = fmt.Sprintf("[%s] %s", remote.Info.Note, description)
			}

			// Create a new tool with prefixed name
			prefixedTool := mcp.NewTool(prefixedName,
				mcp.WithDescription(description),
			)

			// Copy the input schema
			prefixedTool.InputSchema = tool.InputSchema

			// Create handler that proxies to the remote
			handler := a.createProxyHandler(prefixedName)

			result = append(result, tools.ToolDef{
				Tool:    prefixedTool,
				Handler: handler,
			})
		}
	}

	return result
}

// createProxyHandler creates a handler function that proxies calls to the remote server
func (a *ToolAggregator) createProxyHandler(prefixedName string) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		a.mu.RLock()
		source, exists := a.toolMapping[prefixedName]
		a.mu.RUnlock()

		if !exists {
			return mcp.NewToolResultError(fmt.Sprintf("tool %s not found", prefixedName)), nil
		}

		if !source.remote.IsConnected() {
			return mcp.NewToolResultError(fmt.Sprintf("server disconnected for tool %s", prefixedName)), nil
		}

		// Call the remote tool with the original name
		args, ok := request.Params.Arguments.(map[string]any)
		if !ok && request.Params.Arguments != nil {
			return mcp.NewToolResultError("invalid arguments type"), nil
		}
		result, err := source.remote.CallTool(ctx, source.originalName, args)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("remote call failed: %v", err)), nil
		}

		return result, nil
	}
}

// Close closes all remote connections
func (a *ToolAggregator) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	var lastErr error
	for _, remote := range a.remotes {
		if err := remote.Close(); err != nil {
			lastErr = err
		}
	}

	a.remotes = make(map[string]*RemoteClient)
	a.toolMapping = make(map[string]toolSource)

	return lastErr
}

// RemoteCount returns the number of connected remotes
func (a *ToolAggregator) RemoteCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.remotes)
}

// GetRemoteInfos returns information about all connected remotes
func (a *ToolAggregator) GetRemoteInfos() []discovery.ServiceInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var infos []discovery.ServiceInfo
	for _, remote := range a.remotes {
		infos = append(infos, remote.Info)
	}
	return infos
}
