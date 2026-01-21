package server

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/richinsley/jumpboot-mcp/internal/manager"
	"github.com/richinsley/jumpboot-mcp/internal/tools"
)

const (
	ServerName    = "jumpboot-mcp"
	ServerVersion = "1.0.0"
)

// New creates and configures a new MCP server with all tools
func New(mgr *manager.Manager) *server.MCPServer {
	s := server.NewMCPServer(
		ServerName,
		ServerVersion,
		server.WithToolCapabilities(true),
	)

	// Register all tools
	registerTools(s, mgr)

	return s
}

func registerTools(s *server.MCPServer, mgr *manager.Manager) {
	// Collect all tool definitions
	allTools := []tools.ToolDef{}
	allTools = append(allTools, tools.RegisterEnvironmentTools(mgr)...)
	allTools = append(allTools, tools.RegisterPackageTools(mgr)...)
	allTools = append(allTools, tools.RegisterExecutionTools(mgr)...)
	allTools = append(allTools, tools.RegisterREPLTools(mgr)...)
	allTools = append(allTools, tools.RegisterWorkspaceTools(mgr)...)
	allTools = append(allTools, tools.RegisterProcessTools(mgr)...)

	// Register each tool with the server
	for _, td := range allTools {
		s.AddTool(td.Tool, td.Handler)
	}
}

// ServerTools returns all registered tools for inspection
func ServerTools(mgr *manager.Manager) []mcp.Tool {
	allTools := []tools.ToolDef{}
	allTools = append(allTools, tools.RegisterEnvironmentTools(mgr)...)
	allTools = append(allTools, tools.RegisterPackageTools(mgr)...)
	allTools = append(allTools, tools.RegisterExecutionTools(mgr)...)
	allTools = append(allTools, tools.RegisterREPLTools(mgr)...)
	allTools = append(allTools, tools.RegisterWorkspaceTools(mgr)...)
	allTools = append(allTools, tools.RegisterProcessTools(mgr)...)

	result := make([]mcp.Tool, len(allTools))
	for i, td := range allTools {
		result[i] = td.Tool
	}
	return result
}
