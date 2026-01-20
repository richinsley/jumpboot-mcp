package tools

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/richinsley/jumpboot-mcp/internal/manager"
)

// RegisterWorkspaceTools registers workspace management tools with the server
func RegisterWorkspaceTools(mgr *manager.Manager) []ToolDef {
	return []ToolDef{
		{
			Tool: mcp.NewTool("workspace_create",
				mcp.WithDescription("Create a temp code folder (workspace) for an environment"),
				mcp.WithString("env_id", mcp.Required(), mcp.Description("Environment ID")),
			),
			Handler: workspaceCreateHandler(mgr),
		},
		{
			Tool: mcp.NewTool("workspace_write_file",
				mcp.WithDescription("Write a file to the workspace"),
				mcp.WithString("env_id", mcp.Required(), mcp.Description("Environment ID")),
				mcp.WithString("filename", mcp.Required(), mcp.Description("Name of the file to write")),
				mcp.WithString("content", mcp.Required(), mcp.Description("Content to write to the file")),
			),
			Handler: workspaceWriteFileHandler(mgr),
		},
		{
			Tool: mcp.NewTool("workspace_read_file",
				mcp.WithDescription("Read a file from the workspace"),
				mcp.WithString("env_id", mcp.Required(), mcp.Description("Environment ID")),
				mcp.WithString("filename", mcp.Required(), mcp.Description("Name of the file to read")),
			),
			Handler: workspaceReadFileHandler(mgr),
		},
		{
			Tool: mcp.NewTool("workspace_list_files",
				mcp.WithDescription("List files in the workspace or a subdirectory"),
				mcp.WithString("env_id", mcp.Required(), mcp.Description("Environment ID")),
				mcp.WithString("path", mcp.Description("Subdirectory path to list (e.g., 'repo/src'). Defaults to workspace root")),
			),
			Handler: workspaceListFilesHandler(mgr),
		},
		{
			Tool: mcp.NewTool("workspace_delete_file",
				mcp.WithDescription("Delete a file from the workspace"),
				mcp.WithString("env_id", mcp.Required(), mcp.Description("Environment ID")),
				mcp.WithString("filename", mcp.Required(), mcp.Description("Name of the file to delete")),
			),
			Handler: workspaceDeleteFileHandler(mgr),
		},
		{
			Tool: mcp.NewTool("workspace_run_script",
				mcp.WithDescription("Run a Python script from the workspace"),
				mcp.WithString("env_id", mcp.Required(), mcp.Description("Environment ID")),
				mcp.WithString("filename", mcp.Required(), mcp.Description("Name of the script file to run")),
				mcp.WithArray("args",
					mcp.Description("Command-line arguments for the script"),
					mcp.Items(map[string]interface{}{"type": "string"}),
				),
			),
			Handler: workspaceRunScriptHandler(mgr),
		},
		{
			Tool: mcp.NewTool("workspace_destroy",
				mcp.WithDescription("Destroy the workspace (delete all files)"),
				mcp.WithString("env_id", mcp.Required(), mcp.Description("Environment ID")),
			),
			Handler: workspaceDestroyHandler(mgr),
		},
		{
			Tool: mcp.NewTool("workspace_git_clone",
				mcp.WithDescription("Clone a git repository into the workspace"),
				mcp.WithString("env_id", mcp.Required(), mcp.Description("Environment ID")),
				mcp.WithString("repo_url", mcp.Required(), mcp.Description("Git repository URL (https or ssh)")),
				mcp.WithString("dir_name", mcp.Description("Directory name for the clone (defaults to repo name)")),
			),
			Handler: workspaceGitCloneHandler(mgr),
		},
	}
}

func workspaceCreateHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		envID := request.GetString("env_id", "")
		if envID == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingEnvID)), nil
		}

		info, err := mgr.CreateWorkspace(envID)
		if err != nil {
			return mcp.NewToolResultText(manager.ErrorResponse(err)), nil
		}

		return mcp.NewToolResultText(manager.SuccessResponse(info)), nil
	}
}

func workspaceWriteFileHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		envID := request.GetString("env_id", "")
		if envID == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingEnvID)), nil
		}

		filename := request.GetString("filename", "")
		content := request.GetString("content", "")
		if filename == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingParams)), nil
		}

		info, err := mgr.WriteWorkspaceFile(envID, filename, content)
		if err != nil {
			return mcp.NewToolResultText(manager.ErrorResponse(err)), nil
		}

		return mcp.NewToolResultText(manager.SuccessResponse(info)), nil
	}
}

func workspaceReadFileHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		envID := request.GetString("env_id", "")
		if envID == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingEnvID)), nil
		}

		filename := request.GetString("filename", "")
		if filename == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingParams)), nil
		}

		content, err := mgr.ReadWorkspaceFile(envID, filename)
		if err != nil {
			return mcp.NewToolResultText(manager.ErrorResponse(err)), nil
		}

		return mcp.NewToolResultText(manager.SuccessResponse(map[string]string{
			"filename": filename,
			"content":  content,
		})), nil
	}
}

func workspaceListFilesHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		envID := request.GetString("env_id", "")
		if envID == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingEnvID)), nil
		}

		subpath := request.GetString("path", "")

		files, err := mgr.ListWorkspaceFiles(envID, subpath)
		if err != nil {
			return mcp.NewToolResultText(manager.ErrorResponse(err)), nil
		}

		return mcp.NewToolResultText(manager.SuccessResponse(files)), nil
	}
}

func workspaceDeleteFileHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		envID := request.GetString("env_id", "")
		if envID == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingEnvID)), nil
		}

		filename := request.GetString("filename", "")
		if filename == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingParams)), nil
		}

		err := mgr.DeleteWorkspaceFile(envID, filename)
		if err != nil {
			return mcp.NewToolResultText(manager.ErrorResponse(err)), nil
		}

		return mcp.NewToolResultText(manager.SuccessResponse(map[string]string{
			"message":  "File deleted successfully",
			"filename": filename,
		})), nil
	}
}

func workspaceRunScriptHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		envID := request.GetString("env_id", "")
		if envID == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingEnvID)), nil
		}

		filename := request.GetString("filename", "")
		if filename == "" {
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

		output, err := mgr.RunWorkspaceScript(envID, filename, args)
		if err != nil {
			return mcp.NewToolResultText(manager.ErrorResponse(err)), nil
		}

		return mcp.NewToolResultText(manager.SuccessResponse(map[string]string{
			"output": output,
		})), nil
	}
}

func workspaceDestroyHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		envID := request.GetString("env_id", "")
		if envID == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingEnvID)), nil
		}

		err := mgr.DestroyWorkspace(envID)
		if err != nil {
			return mcp.NewToolResultText(manager.ErrorResponse(err)), nil
		}

		return mcp.NewToolResultText(manager.SuccessResponse(map[string]string{
			"message": "Workspace destroyed successfully",
			"env_id":  envID,
		})), nil
	}
}

func workspaceGitCloneHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		envID := request.GetString("env_id", "")
		if envID == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingEnvID)), nil
		}

		repoURL := request.GetString("repo_url", "")
		if repoURL == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingParams)), nil
		}

		dirName := request.GetString("dir_name", "")

		info, err := mgr.GitCloneToWorkspace(envID, repoURL, dirName)
		if err != nil {
			return mcp.NewToolResultText(manager.ErrorResponse(err)), nil
		}

		return mcp.NewToolResultText(manager.SuccessResponse(info)), nil
	}
}
