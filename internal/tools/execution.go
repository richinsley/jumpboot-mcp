package tools

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/richinsley/jumpboot-mcp/internal/manager"
)

// RegisterExecutionTools registers code execution tools with the server
func RegisterExecutionTools(mgr *manager.Manager) []ToolDef {
	return []ToolDef{
		{
			Tool: mcp.NewTool("run_code",
				mcp.WithDescription("Execute a Python code snippet in an environment"),
				mcp.WithString("env_id", mcp.Required(), mcp.Description("Environment ID")),
				mcp.WithString("code", mcp.Required(), mcp.Description("Python code to execute")),
				mcp.WithString("input_json", mcp.Description("Optional JSON input data")),
			),
			Handler: runCodeHandler(mgr),
		},
		{
			Tool: mcp.NewTool("run_script",
				mcp.WithDescription("Execute a Python script file in an environment"),
				mcp.WithString("env_id", mcp.Required(), mcp.Description("Environment ID")),
				mcp.WithString("script_path", mcp.Required(), mcp.Description("Path to Python script")),
				mcp.WithArray("args",
					mcp.Description("Command-line arguments for the script"),
					mcp.Items(map[string]interface{}{"type": "string"}),
				),
			),
			Handler: runScriptHandler(mgr),
		},
	}
}

func runCodeHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		envID := request.GetString("env_id", "")
		if envID == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingEnvID)), nil
		}

		code := request.GetString("code", "")
		if code == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingCode)), nil
		}

		inputJSON := request.GetString("input_json", "")

		output, err := mgr.RunCode(envID, code, inputJSON)
		if err != nil {
			return mcp.NewToolResultText(manager.ErrorResponse(err)), nil
		}

		return mcp.NewToolResultText(manager.SuccessResponse(map[string]string{
			"output": output,
		})), nil
	}
}

func runScriptHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		envID := request.GetString("env_id", "")
		if envID == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingEnvID)), nil
		}

		scriptPath := request.GetString("script_path", "")
		if scriptPath == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingParams)), nil
		}

		// Extract args array from arguments
		var args []string
		reqArgs := request.GetArguments()
		if argsRaw, ok := reqArgs["args"]; ok {
			switch v := argsRaw.(type) {
			case []interface{}:
				for _, a := range v {
					if s, ok := a.(string); ok {
						args = append(args, s)
					}
				}
			case []string:
				args = v
			default:
				data, _ := json.Marshal(argsRaw)
				json.Unmarshal(data, &args)
			}
		}

		output, err := mgr.RunScript(envID, scriptPath, args)
		if err != nil {
			return mcp.NewToolResultText(manager.ErrorResponse(err)), nil
		}

		return mcp.NewToolResultText(manager.SuccessResponse(map[string]string{
			"output": output,
		})), nil
	}
}
