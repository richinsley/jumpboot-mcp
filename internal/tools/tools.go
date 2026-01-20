package tools

import (
	"errors"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Common errors
var (
	errMissingEnvID    = errors.New("env_id is required")
	errMissingParams   = errors.New("missing required parameters")
	errMissingCode     = errors.New("code is required")
	errMissingSessionID = errors.New("session_id is required")
)

// ToolDef pairs a tool with its handler
type ToolDef struct {
	Tool    mcp.Tool
	Handler server.ToolHandlerFunc
}
