package tools

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/richinsley/jumpboot-mcp/internal/manager"
)

// RegisterPackageTools registers package management tools with the server
func RegisterPackageTools(mgr *manager.Manager) []ToolDef {
	return []ToolDef{
		{
			Tool: mcp.NewTool("install_packages",
				mcp.WithDescription("Install Python packages in an environment"),
				mcp.WithString("env_id", mcp.Required(), mcp.Description("Environment ID")),
				mcp.WithArray("packages",
					mcp.Required(),
					mcp.Description("List of packages to install"),
					mcp.Items(map[string]interface{}{"type": "string"}),
				),
				mcp.WithBoolean("use_conda", mcp.Description("Use conda instead of pip. Default: false")),
			),
			Handler: installPackagesHandler(mgr),
		},
		{
			Tool: mcp.NewTool("list_packages",
				mcp.WithDescription("List installed packages in an environment"),
				mcp.WithString("env_id", mcp.Required(), mcp.Description("Environment ID")),
			),
			Handler: listPackagesHandler(mgr),
		},
	}
}

func installPackagesHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		envID := request.GetString("env_id", "")
		if envID == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingEnvID)), nil
		}

		useConda := request.GetBool("use_conda", false)

		// Extract packages array from arguments
		args := request.GetArguments()
		packagesRaw, ok := args["packages"]
		if !ok {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingParams)), nil
		}

		var packages []string
		switch v := packagesRaw.(type) {
		case []interface{}:
			for _, p := range v {
				if s, ok := p.(string); ok {
					packages = append(packages, s)
				}
			}
		case []string:
			packages = v
		default:
			// Try JSON unmarshaling
			data, _ := json.Marshal(packagesRaw)
			json.Unmarshal(data, &packages)
		}

		if len(packages) == 0 {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingParams)), nil
		}

		err := mgr.InstallPackages(envID, packages, useConda)
		if err != nil {
			return mcp.NewToolResultText(manager.ErrorResponse(err)), nil
		}

		return mcp.NewToolResultText(manager.SuccessResponse(map[string]interface{}{
			"message":  "Packages installed successfully",
			"packages": packages,
		})), nil
	}
}

func listPackagesHandler(mgr *manager.Manager) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		envID := request.GetString("env_id", "")
		if envID == "" {
			return mcp.NewToolResultText(manager.ErrorResponse(errMissingEnvID)), nil
		}

		packages, err := mgr.ListPackages(envID)
		if err != nil {
			return mcp.NewToolResultText(manager.ErrorResponse(err)), nil
		}

		return mcp.NewToolResultText(manager.SuccessResponse(packages)), nil
	}
}
