package tools

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/richinsley/jumpboot-mcp/internal/discovery"
)

// RemoteServerProvider is implemented by the aggregator to provide remote server info
type RemoteServerProvider interface {
	GetRemoteInfos() []discovery.ServiceInfo
}

// RegisterFederationTools registers tools for managing federated servers
func RegisterFederationTools(provider RemoteServerProvider) []ToolDef {
	if provider == nil {
		return nil
	}

	return []ToolDef{
		{
			Tool: mcp.NewTool("list_servers",
				mcp.WithDescription("List all discovered remote jumpboot-mcp servers"),
			),
			Handler: func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				infos := provider.GetRemoteInfos()

				type serverInfo struct {
					InstanceName string `json:"instance_name"`
					URL          string `json:"url"`
					Note         string `json:"note,omitempty"`
				}

				servers := make([]serverInfo, len(infos))
				for i, info := range infos {
					servers[i] = serverInfo{
						InstanceName: info.InstanceName,
						URL:          info.URL(),
						Note:         info.Note,
					}
				}

				result := map[string]any{
					"success": true,
					"data": map[string]any{
						"servers": servers,
						"count":   len(servers),
					},
					"error": nil,
				}

				jsonBytes, _ := json.MarshalIndent(result, "", "  ")
				return mcp.NewToolResultText(string(jsonBytes)), nil
			},
		},
	}
}
