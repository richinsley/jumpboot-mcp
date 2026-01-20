package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/richinsley/jumpboot-mcp/internal/manager"
)

// RegisterREPLTools registers REPL session tools with the server
func RegisterREPLTools(mgr *manager.Manager) []ToolDef {
	return []ToolDef{
		{
			Tool: mcp.NewTool("repl_create",
				mcp.WithDescription("Create a persistent REPL session for an environment"),
				mcp.WithString("env_id", mcp.Required(), mcp.Description("Environment ID")),
				mcp.WithString("session_name", mcp.Required(), mcp.Description("Name for the REPL session")),
			),
			Handler: replCreateHandler(mgr),
		},
		{
			Tool: mcp.NewTool("repl_execute",
				mcp.WithDescription("Execute code in a REPL session (state is preserved between calls)"),
				mcp.WithString("session_id", mcp.Required(), mcp.Description("REPL session ID")),
				mcp.WithString("code", mcp.Required(), mcp.Description("Python code to execute")),
			),
			Handler: replExecuteHandler(mgr),
		},
		{
			Tool: mcp.NewTool("repl_list",
				mcp.WithDescription("List all active REPL sessions"),
			),
			Handler: replListHandler(mgr),
		},
		{
			Tool: mcp.NewTool("repl_destroy",
				mcp.WithDescription("Close and destroy a REPL session"),
				mcp.WithString("session_id", mcp.Required(), mcp.Description("REPL session ID to destroy")),
			),
			Handler: replDestroyHandler(mgr),
		},
	}
}

func replCreateHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		envID := request.GetString("env_id", "")
		if envID == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingEnvID)), nil
		}

		sessionName := request.GetString("session_name", "")
		if sessionName == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingParams)), nil
		}

		info, err := mgr.CreateREPL(envID, sessionName)
		if err != nil {
			return mcp.NewToolResultText(manager.ErrorResponse(err)), nil
		}

		return mcp.NewToolResultText(manager.SuccessResponse(info)), nil
	}
}

func replExecuteHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sessionID := request.GetString("session_id", "")
		if sessionID == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingSessionID)), nil
		}

		code := request.GetString("code", "")
		if code == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingCode)), nil
		}

		output, err := mgr.ExecuteREPL(sessionID, code)
		if err != nil {
			return mcp.NewToolResultText(manager.ErrorResponse(err)), nil
		}

		return mcp.NewToolResultText(manager.SuccessResponse(map[string]string{
			"output": output,
		})), nil
	}
}

func replListHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sessions := mgr.ListREPLs()
		return mcp.NewToolResultText(manager.SuccessResponse(sessions)), nil
	}
}

func replDestroyHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sessionID := request.GetString("session_id", "")
		if sessionID == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingSessionID)), nil
		}

		err := mgr.DestroyREPL(sessionID)
		if err != nil {
			return mcp.NewToolResultText(manager.ErrorResponse(err)), nil
		}

		return mcp.NewToolResultText(manager.SuccessResponse(map[string]string{
			"message":    "REPL session destroyed successfully",
			"session_id": sessionID,
		})), nil
	}
}
