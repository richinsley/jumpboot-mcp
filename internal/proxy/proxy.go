package proxy

import (
	"context"
	"fmt"
	"sync"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/richinsley/jumpboot-mcp/internal/discovery"
)

// RemoteClient wraps an MCP client connection to a remote jumpboot-mcp server
type RemoteClient struct {
	Info   discovery.ServiceInfo
	client *client.Client
	tools  []mcp.Tool
	mu     sync.RWMutex
}

// NewRemoteClient creates a new remote client for the given service
func NewRemoteClient(info discovery.ServiceInfo) *RemoteClient {
	return &RemoteClient{
		Info: info,
	}
}

// Connect establishes a connection to the remote MCP server
func (r *RemoteClient) Connect(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Create HTTP client
	url := r.Info.URL()
	c, err := client.NewStreamableHttpClient(url)
	if err != nil {
		return fmt.Errorf("failed to create client for %s: %w", url, err)
	}

	// Start the client
	if err := c.Start(ctx); err != nil {
		return fmt.Errorf("failed to start client for %s: %w", url, err)
	}

	// Initialize the connection
	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initReq.Params.ClientInfo = mcp.Implementation{
		Name:    "jumpboot-mcp-proxy",
		Version: "1.0.0",
	}

	_, err = c.Initialize(ctx, initReq)
	if err != nil {
		c.Close()
		return fmt.Errorf("failed to initialize connection to %s: %w", url, err)
	}

	r.client = c

	// Fetch tools from the remote server
	if err := r.fetchTools(ctx); err != nil {
		c.Close()
		return fmt.Errorf("failed to fetch tools from %s: %w", url, err)
	}

	return nil
}

// fetchTools retrieves the list of tools from the remote server
func (r *RemoteClient) fetchTools(ctx context.Context) error {
	toolsReq := mcp.ListToolsRequest{}
	result, err := r.client.ListTools(ctx, toolsReq)
	if err != nil {
		return err
	}

	r.tools = result.Tools
	return nil
}

// Tools returns the tools available from this remote server
func (r *RemoteClient) Tools() []mcp.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.tools
}

// CallTool invokes a tool on the remote server
func (r *RemoteClient) CallTool(ctx context.Context, name string, args map[string]any) (*mcp.CallToolResult, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.client == nil {
		return nil, fmt.Errorf("client not connected")
	}

	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args

	return r.client.CallTool(ctx, req)
}

// Close closes the connection to the remote server
func (r *RemoteClient) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.client != nil {
		err := r.client.Close()
		r.client = nil
		return err
	}
	return nil
}

// IsConnected returns true if the client is connected
func (r *RemoteClient) IsConnected() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.client != nil
}
