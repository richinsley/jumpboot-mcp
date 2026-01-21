package tools

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/richinsley/jumpboot-mcp/internal/manager"
)

// RegisterProcessTools registers process management tools with the server
func RegisterProcessTools(mgr *manager.Manager) []ToolDef {
	return []ToolDef{
		{
			Tool: mcp.NewTool("spawn_process",
				mcp.WithDescription("Spawn a Python script that runs in the background. Use for long-running tasks like GUI apps, servers, or games."),
				mcp.WithString("env_id", mcp.Required(), mcp.Description("Environment ID")),
				mcp.WithString("script_path", mcp.Required(), mcp.Description("Path to the Python script (relative to workspace)")),
				mcp.WithString("name", mcp.Description("Name for the process (defaults to script filename)")),
				mcp.WithArray("args",
					mcp.Description("Command-line arguments for the script"),
					mcp.Items(map[string]interface{}{"type": "string"}),
				),
				mcp.WithBoolean("capture_output", mcp.Description("Capture stdout/stderr for later retrieval. Set false for GUI apps. Default: true")),
			),
			Handler: spawnProcessHandler(mgr),
		},
		{
			Tool: mcp.NewTool("list_processes",
				mcp.WithDescription("List all spawned processes"),
			),
			Handler: listProcessesHandler(mgr),
		},
		{
			Tool: mcp.NewTool("process_output",
				mcp.WithDescription("Get stdout/stderr output from a spawned process"),
				mcp.WithString("process_id", mcp.Required(), mcp.Description("Process ID")),
				mcp.WithNumber("tail_lines", mcp.Description("Number of lines to return from the end. Default: all lines")),
			),
			Handler: processOutputHandler(mgr),
		},
		{
			Tool: mcp.NewTool("kill_process",
				mcp.WithDescription("Terminate a spawned process"),
				mcp.WithString("process_id", mcp.Required(), mcp.Description("Process ID to kill")),
			),
			Handler: killProcessHandler(mgr),
		},
	}
}

func spawnProcessHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		envID := request.GetString("env_id", "")
		if envID == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingEnvID)), nil
		}

		scriptPath := request.GetString("script_path", "")
		if scriptPath == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingParams)), nil
		}

		name := request.GetString("name", "")
		captureOutput := request.GetBool("capture_output", true)

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

		info, err := mgr.SpawnProcess(envID, scriptPath, name, args, captureOutput)
		if err != nil {
			return mcp.NewToolResultText(manager.ErrorResponse(err)), nil
		}

		return mcp.NewToolResultText(manager.SuccessResponse(info)), nil
	}
}

func listProcessesHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		processes := mgr.ListProcesses()
		return mcp.NewToolResultText(manager.SuccessResponse(processes)), nil
	}
}

func processOutputHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		processID := request.GetString("process_id", "")
		if processID == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingParams)), nil
		}

		// Extract tail_lines from arguments
		tailLines := 0
		args := request.GetArguments()
		if v, ok := args["tail_lines"]; ok {
			switch n := v.(type) {
			case float64:
				tailLines = int(n)
			case int:
				tailLines = n
			}
		}

		lines, err := mgr.GetProcessOutput(processID, tailLines)
		if err != nil {
			return mcp.NewToolResultText(manager.ErrorResponse(err)), nil
		}

		// Get process info for status
		info, _ := mgr.GetProcessInfo(processID)

		return mcp.NewToolResultText(manager.SuccessResponse(map[string]interface{}{
			"process_id": processID,
			"output":     strings.Join(lines, "\n"),
			"lines":      len(lines),
			"running":    info != nil && info.Running,
		})), nil
	}
}

func killProcessHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		processID := request.GetString("process_id", "")
		if processID == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingParams)), nil
		}

		err := mgr.KillProcess(processID)
		if err != nil {
			return mcp.NewToolResultText(manager.ErrorResponse(err)), nil
		}

		return mcp.NewToolResultText(manager.SuccessResponse(map[string]string{
			"message":    "Process terminated successfully",
			"process_id": processID,
		})), nil
	}
}
