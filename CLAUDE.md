# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

MCP (Model Context Protocol) server in Go that wraps the [Jumpboot](https://github.com/richinsley/jumpboot) library, exposing Python environment management capabilities to AI assistants. All environments are independent of system Python.

## Build & Run Commands

```bash
# Build the server
go build -o jumpboot-mcp .

# Run the server (stdio transport - default)
./jumpboot-mcp

# Run the server (HTTP transport for containers)
./jumpboot-mcp -transport http -addr :8080 -endpoint /mcp

# Run with HTTPS
./jumpboot-mcp -transport http -addr :8443 -tls-cert cert.pem -tls-key key.pem

# Docker build and run
docker build -t jumpboot-mcp .
docker run -p 8080:8080 -v jumpboot-data:/root/.jumpboot-mcp jumpboot-mcp

# Run tests
go test ./...

# Run a single test
go test -run TestName ./internal/tools/

# Add dependencies
go get github.com/richinsley/jumpboot
go get github.com/mark3labs/mcp-go
```

## Transport Options

| Flag | Default | Description |
|------|---------|-------------|
| `-transport` | `stdio` | Transport type: `stdio` or `http` |
| `-addr` | `:8080` | HTTP server address |
| `-endpoint` | `/mcp` | HTTP endpoint path |
| `-stateless` | `false` | Stateless mode (no session tracking) |
| `-tls-cert` | | TLS certificate file |
| `-tls-key` | | TLS key file |

## mDNS Service Discovery Options

| Flag | Default | Description |
|------|---------|-------------|
| `-note` | `""` | Human-readable server description (e.g., "GPU server for ML") |
| `-instance-name` | hostname | Unique mDNS instance name |
| `-mdns-announce` | `true` (HTTP mode) | Enable mDNS service announcement |
| `-mdns-discover` | `true` (stdio mode) | Enable mDNS service discovery |
| `-discover-timeout` | `5s` | Discovery wait time at startup |

### mDNS Discovery Flow

**HTTP mode** (server):
- Announces service via mDNS with type `_jumpboot-mcp._tcp`
- TXT records include: `endpoint`, `tls`, `note`
- Other stdio instances can discover and proxy to this server

**Stdio mode** (client):
- Discovers HTTP instances on local network via mDNS
- Connects to each discovered server
- Proxies remote tools with prefixed names (e.g., `gpu-server:create_environment`)
- Tool descriptions include the server's note (e.g., "[GPU server for ML] Create a new...")

### Example: Distributed Setup

```bash
# On GPU server (machine A)
./jumpboot-mcp -transport http -addr :9999 -note "GPU server for ML" -instance-name gpu-server

# On local machine (machine B) - Claude Desktop uses this
./jumpboot-mcp
# Discovers gpu-server automatically, registers tools like:
#   gpu-server:create_environment
#   gpu-server:run_code
#   etc.
```

## Architecture

**Transport**: stdio (standard MCP transport) or HTTP (for containers/remote)

**Core Components**:
- `main.go` - Entry point, MCP server initialization, mDNS integration
- `internal/server/server.go` - MCP server configuration and tool registration
- `internal/manager/manager.go` - Stateful environment manager (tracks environments by UUID, REPL sessions, handles cleanup)
- `internal/tools/` - MCP tool implementations:
  - `environment.go` - create/list/destroy/freeze/restore environments
  - `packages.go` - pip/conda package installation, requirements.txt support
  - `execution.go` - code/script execution
  - `repl.go` - persistent REPL session management
  - `workspace.go` - persistent code folder management
  - `process.go` - long-running process management (GUI apps, servers, games)
- `internal/discovery/` - mDNS service discovery:
  - `discovery.go` - ServiceInfo type, constants
  - `announce.go` - mDNS announcer for HTTP mode
  - `browser.go` - mDNS browser for stdio mode
- `internal/proxy/` - Remote MCP client proxy:
  - `proxy.go` - RemoteClient wraps mcp-go HTTP client
  - `aggregator.go` - Aggregates tools from multiple remotes

**Data Flow**:
- Local: MCP Client → stdio → Server → Manager → Jumpboot Library → Python Environment
- Distributed: MCP Client → stdio → Server → Proxy → HTTP → Remote Server → Manager → Python Environment

## Data Storage

All data stored in `~/.jumpboot-mcp/envs/`:

```
~/.jumpboot-mcp/envs/
├── bases/                    # Cached micromamba base environments
│   ├── base_3.11/           # Base for Python 3.11
│   └── base_3.12/           # Base for Python 3.12
└── {env-uuid}/              # User environments (venvs)
    ├── bin/
    ├── lib/
    ├── pyvenv.cfg
    └── workspace/           # Persistent workspace
```

**Environment Creation Strategy**:
- Default Python version: `3.11`
- All environments are venvs created from cached micromamba bases (independent of system Python)
- First environment for a version creates base via micromamba (slow, downloads Python)
- Subsequent environments for same version use fast venv creation from cached base

## Tool Response Format

All MCP tools return:
```json
{"success": true, "data": {...}, "error": null}
```
Or on error:
```json
{"success": false, "data": null, "error": "message"}
```

## Key Jumpboot API Patterns (v1.0.0)

```go
// All functions return *jumpboot.PythonEnvironment (renamed from Environment in v1.0.0)

// Create base environment with micromamba
baseEnv, err := jumpboot.CreateEnvironmentMamba(name, rootDir, pythonVersion, "conda-forge", nil)

// Create venv from base environment
env, err := jumpboot.CreateVenvEnvironment(baseEnv, venvPath, jumpboot.VenvOptions{}, nil)

// Package management
env.PipInstallPackages([]string{"numpy", "pandas"}, "", "", false, nil)
env.MicromambaInstallPackage("scipy", "conda-forge")

// Install from requirements.txt (use environment's pip)
env.RunPythonReadCombined("-m", "pip", "install", "-r", "requirements.txt")

// Code execution
output, err := env.RunPythonReadCombined(scriptPath)
output, err := env.RunPythonReadStdout("-m", "pip", "freeze")

// REPL (persistent sessions)
repl, err := env.NewREPLPythonProcess(kvpairs, env_vars, modules, packages)
result, err := repl.Execute(code, true)  // true = combined output
result, err := repl.ExecuteWithTimeout(code, true, 30*time.Second)  // with timeout
repl.Close()

// Freeze/restore
env.FreezeToFile(filePath)
env, err := jumpboot.CreateEnvironmentFromJSONFile(jsonPath, rootDir, nil)
env, err := jumpboot.CreateEnvironmentFromJSONFileWithOptions(jsonPath, rootDir, opts, nil)
```

## MCP Tools Reference (26 tools)

### Environment Management
| Tool | Parameters |
|------|------------|
| `create_environment` | `name`, `python_version` |
| `list_environments` | none |
| `destroy_environment` | `env_id` |
| `freeze_environment` | `env_id` |
| `restore_environment` | `name`, `frozen_json` |

### Package Management
| Tool | Parameters |
|------|------------|
| `install_packages` | `env_id`, `packages[]`, `use_conda` |
| `install_requirements` | `env_id`, `requirements_path`, `upgrade` (optional) |
| `list_packages` | `env_id` |

### Code Execution
| Tool | Parameters |
|------|------------|
| `run_code` | `env_id`, `code`, `input_json` |
| `run_script` | `env_id`, `script_path`, `args[]` |

### REPL Sessions
| Tool | Parameters |
|------|------------|
| `repl_create` | `env_id`, `session_name` |
| `repl_execute` | `session_id`, `code` |
| `repl_list` | none |
| `repl_destroy` | `session_id` |

### Workspace Management
| Tool | Parameters |
|------|------------|
| `workspace_create` | `env_id` |
| `workspace_write_file` | `env_id`, `filename`, `content` |
| `workspace_read_file` | `env_id`, `filename` |
| `workspace_list_files` | `env_id`, `path` (optional subdir) |
| `workspace_delete_file` | `env_id`, `filename` |
| `workspace_run_script` | `env_id`, `filename`, `args[]` |
| `workspace_git_clone` | `env_id`, `repo_url`, `dir_name` (optional) |
| `workspace_destroy` | `env_id` |

### Process Management (Long-running)
| Tool | Parameters |
|------|------------|
| `spawn_process` | `env_id`, `script_path`, `name`, `args[]`, `capture_output` |
| `list_processes` | none |
| `process_output` | `process_id`, `tail_lines` (optional) |
| `kill_process` | `process_id` |

## Claude Desktop Configuration

Add to `~/.config/claude/claude_desktop_config.json`:
```json
{
  "mcpServers": {
    "jumpboot": {
      "command": "/path/to/jumpboot-mcp",
      "args": []
    }
  }
}
```
