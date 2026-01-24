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
	return NewWithExtraTools(mgr, nil)
}

// NewWithExtraTools creates a new MCP server with local tools plus additional tools (e.g., proxied remote tools)
func NewWithExtraTools(mgr *manager.Manager, extraTools []tools.ToolDef) *server.MCPServer {
	s := server.NewMCPServer(
		ServerName,
		ServerVersion,
		server.WithToolCapabilities(true),
	)

	// Register all local tools
	// If we have remote tools, prefix local tool descriptions with "[local]"
	hasRemoteTools := len(extraTools) > 0
	registerToolsWithPrefix(s, mgr, hasRemoteTools)

	// Register extra tools (e.g., proxied remote tools)
	for _, td := range extraTools {
		s.AddTool(td.Tool, td.Handler)
	}

	return s
}

func registerToolsWithPrefix(s *server.MCPServer, mgr *manager.Manager, addLocalPrefix bool) {
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
		if addLocalPrefix {
			// Create a copy with "[local]" prefix in description
			localTool := mcp.NewTool(td.Tool.Name,
				mcp.WithDescription("[local] "+td.Tool.Description),
			)
			localTool.InputSchema = td.Tool.InputSchema
			s.AddTool(localTool, td.Handler)
		} else {
			s.AddTool(td.Tool, td.Handler)
		}
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
