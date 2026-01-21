package tools

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/richinsley/jumpboot-mcp/internal/manager"
)

// RegisterEnvironmentTools registers environment management tools with the server
func RegisterEnvironmentTools(mgr *manager.Manager) []ToolDef {
	return []ToolDef{
		{
			Tool: mcp.NewTool("create_environment",
				mcp.WithDescription("Create a new Python environment. Creates a venv from a cached micromamba base (independent of system Python). First call for a Python version creates the base (slower), subsequent calls are fast."),
				mcp.WithString("name", mcp.Required(), mcp.Description("Name for the environment")),
				mcp.WithString("python_version", mcp.Description("Python version (e.g., '3.11'). Default: '3.11'")),
			),
			Handler: createEnvironmentHandler(mgr),
		},
		{
			Tool: mcp.NewTool("list_environments",
				mcp.WithDescription("List all managed Python environments"),
			),
			Handler: listEnvironmentsHandler(mgr),
		},
		{
			Tool: mcp.NewTool("destroy_environment",
				mcp.WithDescription("Delete a Python environment"),
				mcp.WithString("env_id", mcp.Required(), mcp.Description("Environment ID to destroy")),
			),
			Handler: destroyEnvironmentHandler(mgr),
		},
		{
			Tool: mcp.NewTool("freeze_environment",
				mcp.WithDescription("Export an environment to JSON for later restoration"),
				mcp.WithString("env_id", mcp.Required(), mcp.Description("Environment ID to freeze")),
			),
			Handler: freezeEnvironmentHandler(mgr),
		},
		{
			Tool: mcp.NewTool("restore_environment",
				mcp.WithDescription("Recreate an environment from frozen JSON"),
				mcp.WithString("name", mcp.Required(), mcp.Description("Name for the restored environment")),
				mcp.WithString("frozen_json", mcp.Required(), mcp.Description("Frozen environment JSON")),
			),
			Handler: restoreEnvironmentHandler(mgr),
		},
	}
}

func createEnvironmentHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := request.GetString("name", "")
		pythonVersion := request.GetString("python_version", "3.11")

		info, err := mgr.CreateEnvironment(name, pythonVersion)
		if err != nil {
			return mcp.NewToolResultText(manager.ErrorResponse(err)), nil
		}

		return mcp.NewToolResultText(manager.SuccessResponse(info)), nil
	}
}

func listEnvironmentsHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		envs := mgr.ListEnvironments()
		return mcp.NewToolResultText(manager.SuccessResponse(envs)), nil
	}
}

func destroyEnvironmentHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		envID := request.GetString("env_id", "")
		if envID == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingEnvID)), nil
		}

		err := mgr.DestroyEnvironment(envID)
		if err != nil {
			return mcp.NewToolResultText(manager.ErrorResponse(err)), nil
		}

		return mcp.NewToolResultText(manager.SuccessResponse(map[string]string{
			"message": "Environment destroyed successfully",
			"env_id":  envID,
		})), nil
	}
}

func freezeEnvironmentHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		envID := request.GetString("env_id", "")
		if envID == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingEnvID)), nil
		}

		frozen, err := mgr.FreezeEnvironment(envID)
		if err != nil {
			return mcp.NewToolResultText(manager.ErrorResponse(err)), nil
		}

		return mcp.NewToolResultText(manager.SuccessResponse(map[string]string{
			"env_id":      envID,
			"frozen_json": frozen,
		})), nil
	}
}

func restoreEnvironmentHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := request.GetString("name", "")
		frozenJSON := request.GetString("frozen_json", "")

		if name == "" || frozenJSON == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingParams)), nil
		}

		info, err := mgr.RestoreEnvironment(name, frozenJSON)
		if err != nil {
			return mcp.NewToolResultText(manager.ErrorResponse(err)), nil
		}

		return mcp.NewToolResultText(manager.SuccessResponse(info)), nil
	}
}
