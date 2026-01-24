# Jumpboot MCP Server

An MCP (Model Context Protocol) server that wraps the [Jumpboot](https://github.com/richinsley/jumpboot) Go library, enabling AI assistants to dynamically create and manage Python environments, install packages, and execute Python code.

## Features

- **Environment Management**: Create isolated Python environments (completely independent of system Python)
- **Package Installation**: Install packages via pip or conda, or from requirements.txt files
- **Code Execution**: Run Python code snippets or script files
- **REPL Sessions**: Maintain persistent Python REPL sessions with preserved state
- **Workspace Management**: Persistent code folders for writing files, cloning repos, and executing scripts
- **Long-running Processes**: Spawn GUI apps, servers, games, and other persistent Python processes
- **Environment Portability**: Freeze and restore environments for reproducibility
- **Server Federation**: Discover and proxy to remote jumpboot-mcp servers via mDNS

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Transport Options](#transport-options)
- [Server Federation (mDNS)](#server-federation-mdns)
- [Docker Deployment](#docker-deployment)
- [Claude Desktop Configuration](#claude-desktop-configuration)
- [Claude Code Configuration](#claude-code-configuration)
- [MCP Tools Reference](#mcp-tools-reference)
- [Usage Examples](#usage-examples)

## Installation

### Prerequisites

- Go 1.21+
- Git

### Build from Source

```bash
git clone https://github.com/richinsley/jumpboot-mcp.git
cd jumpboot-mcp
go build -o jumpboot-mcp .
```

## Quick Start

### Local Usage (stdio)

Run the server locally for Claude Desktop or Claude Code:

```bash
./jumpboot-mcp
```

### HTTP Server

Run as an HTTP server for remote access or containers:

```bash
./jumpboot-mcp -transport http -addr :8080
```

### Federated Setup

Run an HTTP server that announces itself on the network:

```bash
# On a GPU server
./jumpboot-mcp -transport http -addr :8080 -note "GPU server for ML"
```

Then run a local stdio instance that discovers and proxies to it:

```bash
# On your local machine - discovers gpu-server automatically
./jumpboot-mcp
```

## Transport Options

| Flag | Default | Description |
|------|---------|-------------|
| `-transport` | `stdio` | Transport type: `stdio` or `http` |
| `-addr` | `:8080` | HTTP server address |
| `-endpoint` | `/mcp` | HTTP endpoint path |
| `-stateless` | `false` | Run in stateless mode (no session tracking) |
| `-tls-cert` | | TLS certificate file (enables HTTPS) |
| `-tls-key` | | TLS key file (enables HTTPS) |

## Server Federation (mDNS)

Jumpboot-mcp supports automatic service discovery via mDNS (Bonjour/Avahi). This enables a powerful federation model where:

1. **HTTP servers** announce themselves on the local network
2. **Stdio clients** discover HTTP servers and proxy their tools
3. **Claude** sees all tools (local + remote) with prefixed names

### mDNS Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-note` | `""` | Human-readable server description (e.g., "GPU server for ML") |
| `-instance-name` | hostname | Unique mDNS instance name (used as tool prefix) |
| `-mdns-announce` | `true` | Enable mDNS announcement (HTTP mode only) |
| `-mdns-discover` | `true` | Enable mDNS discovery (stdio mode only) |
| `-discover-timeout` | `5s` | How long to wait for discovery at startup |

### How Federation Works

```
┌─────────────────┐     mDNS Discovery      ┌─────────────────────┐
│  Claude Desktop │ ◄──────────────────────►│  HTTP Server A      │
│  or Claude Code │                         │  (gpu-server)       │
│                 │     HTTP/MCP Protocol   │  -note "GPU for ML" │
│  ┌───────────┐  │ ◄─────────────────────► │                     │
│  │ stdio     │  │                         └─────────────────────┘
│  │ jumpboot  │  │
│  │ -mcp      │  │     mDNS Discovery      ┌─────────────────────┐
│  │           │  │ ◄──────────────────────►│  HTTP Server B      │
│  │ (local +  │  │                         │  (pi-cluster)       │
│  │  proxied  │  │     HTTP/MCP Protocol   │  -note "Raspberry"  │
│  │  tools)   │  │ ◄─────────────────────► │                     │
│  └───────────┘  │                         └─────────────────────┘
└─────────────────┘

Tools visible to Claude:
  - create_environment        (local)
  - run_code                  (local)
  - gpu-server:create_environment   (proxied)
  - gpu-server:run_code             (proxied)
  - pi-cluster:create_environment   (proxied)
  - pi-cluster:run_code             (proxied)
```

### Tool Naming Convention

Remote tools are prefixed with the server's instance name:

| Original Tool | Proxied Tool Name |
|--------------|-------------------|
| `create_environment` | `gpu-server:create_environment` |
| `run_code` | `gpu-server:run_code` |
| `install_packages` | `gpu-server:install_packages` |

Tool descriptions are enhanced with the server's note:
- Original: `"Create a new Python environment"`
- Proxied: `"[GPU server for ML] Create a new Python environment"`

### Federation Setup Examples

#### Example 1: GPU Server + Local Machine

**On the GPU server (machine with CUDA):**
```bash
./jumpboot-mcp \
  -transport http \
  -addr :8080 \
  -instance-name gpu-server \
  -note "GPU server with CUDA for ML workloads"
```

**On your local machine:**
```bash
# Just run normally - it discovers gpu-server automatically
./jumpboot-mcp

# Output:
# Discovered 1 remote jumpboot-mcp service(s):
#   - gpu-server at http://192.168.1.100:8080/mcp (GPU server with CUDA for ML workloads)
#     Connected successfully
# Registered 26 proxied tools from remote servers
```

Now Claude can use both local tools and `gpu-server:*` tools.

#### Example 2: Multiple Specialized Servers

**Server 1 - ML workloads:**
```bash
./jumpboot-mcp -transport http -addr :8080 \
  -instance-name ml-server \
  -note "ML server with PyTorch/TensorFlow"
```

**Server 2 - Data processing:**
```bash
./jumpboot-mcp -transport http -addr :8080 \
  -instance-name data-server \
  -note "Data server with Pandas/Spark"
```

**Server 3 - Web scraping:**
```bash
./jumpboot-mcp -transport http -addr :8080 \
  -instance-name scraper \
  -note "Scraping server with Selenium/Playwright"
```

**Local stdio client:**
```bash
./jumpboot-mcp
# Discovers all three servers, registers:
#   - Local tools (26)
#   - ml-server:* tools (26)
#   - data-server:* tools (26)
#   - scraper:* tools (26)
# Total: 104 tools available to Claude
```

#### Example 3: Disable Discovery/Announcement

**HTTP server without mDNS (manual configuration only):**
```bash
./jumpboot-mcp -transport http -addr :8080 -mdns-announce=false
```

**Stdio client without discovery (local tools only):**
```bash
./jumpboot-mcp -mdns-discover=false
```

### Verifying mDNS Announcement

**On macOS:**
```bash
dns-sd -B _jumpboot-mcp._tcp
```

**On Linux (with avahi):**
```bash
avahi-browse -r _jumpboot-mcp._tcp
```

## Docker Deployment

### Build the Image

```bash
docker build -t jumpboot-mcp .
```

### Basic HTTP Server

```bash
docker run -p 8080:8080 jumpboot-mcp
```

### With Persistent Storage (Recommended)

```bash
docker run -p 8080:8080 \
  -v jumpboot-data:/root/.jumpboot-mcp \
  jumpboot-mcp
```

The volume persists cached micromamba bases and environments across container restarts.

### With mDNS Announcement

For mDNS to work in Docker, use host networking:

```bash
docker run --network host \
  -v jumpboot-data:/root/.jumpboot-mcp \
  jumpboot-mcp \
  -transport http -addr :8080 \
  -instance-name docker-server \
  -note "Docker container with Python environments"
```

> **Note:** `--network host` is required for mDNS multicast to work. On macOS Docker Desktop, host networking has limitations - consider running the binary directly instead.

### With HTTPS

```bash
docker run -p 8443:8443 \
  -v jumpboot-data:/root/.jumpboot-mcp \
  -v /path/to/certs:/certs:ro \
  jumpboot-mcp \
  -transport http -addr :8443 \
  -tls-cert /certs/cert.pem \
  -tls-key /certs/key.pem
```

### Docker Compose

#### Basic HTTP Server

```yaml
version: '3.8'
services:
  jumpboot-mcp:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - jumpboot-data:/root/.jumpboot-mcp
    restart: unless-stopped

volumes:
  jumpboot-data:
```

#### With mDNS (Host Networking)

```yaml
version: '3.8'
services:
  jumpboot-mcp:
    build: .
    network_mode: host
    volumes:
      - jumpboot-data:/root/.jumpboot-mcp
    command:
      - "-transport"
      - "http"
      - "-addr"
      - ":8080"
      - "-instance-name"
      - "docker-jumpboot"
      - "-note"
      - "Docker Python environment server"
    restart: unless-stopped

volumes:
  jumpboot-data:
```

#### Multiple Servers

```yaml
version: '3.8'
services:
  ml-server:
    build: .
    network_mode: host
    volumes:
      - ml-data:/root/.jumpboot-mcp
    command: ["-transport", "http", "-addr", ":8080", "-instance-name", "ml-server", "-note", "ML workloads"]

  data-server:
    build: .
    network_mode: host
    volumes:
      - data-data:/root/.jumpboot-mcp
    command: ["-transport", "http", "-addr", ":8081", "-instance-name", "data-server", "-note", "Data processing"]

volumes:
  ml-data:
  data-data:
```

## Claude Desktop Configuration

Claude Desktop uses stdio transport. Configure in:
- **macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
- **Windows**: `%APPDATA%\Claude\claude_desktop_config.json`
- **Linux**: `~/.config/claude/claude_desktop_config.json`

### Local Only (No Federation)

```json
{
  "mcpServers": {
    "jumpboot": {
      "command": "/path/to/jumpboot-mcp",
      "args": ["-mdns-discover=false"]
    }
  }
}
```

### With Federation (Recommended)

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

This will:
1. Start local jumpboot-mcp in stdio mode
2. Discover any HTTP jumpboot-mcp servers on the network via mDNS
3. Proxy remote tools with prefixed names (e.g., `gpu-server:run_code`)

### Custom Discovery Timeout

If discovery is slow on your network:

```json
{
  "mcpServers": {
    "jumpboot": {
      "command": "/path/to/jumpboot-mcp",
      "args": ["-discover-timeout", "10s"]
    }
  }
}
```

### Multiple Explicit Servers (No mDNS)

If you can't use mDNS, configure multiple servers directly:

```json
{
  "mcpServers": {
    "jumpboot-local": {
      "command": "/path/to/jumpboot-mcp",
      "args": ["-mdns-discover=false"]
    }
  }
}
```

> **Note:** Claude Desktop doesn't support HTTP transport directly. For HTTP servers without mDNS, use the federation approach with a local stdio instance that discovers them.

## Claude Code Configuration

Claude Code supports both stdio and HTTP transports.

### Configure via CLI

**Add local stdio server:**
```bash
claude mcp add jumpboot /path/to/jumpboot-mcp
```

**Add local stdio with federation:**
```bash
claude mcp add jumpboot /path/to/jumpboot-mcp
# Federation happens automatically via mDNS
```

**Add remote HTTP server directly:**
```bash
claude mcp add jumpboot-gpu --transport http http://gpu-server:8080/mcp
```

### Configure via .mcp.json

Create `.mcp.json` in your project root or home directory:

#### Local with Federation (Recommended)

```json
{
  "mcpServers": {
    "jumpboot": {
      "command": "/path/to/jumpboot-mcp"
    }
  }
}
```

#### Local Only (No Discovery)

```json
{
  "mcpServers": {
    "jumpboot": {
      "command": "/path/to/jumpboot-mcp",
      "args": ["-mdns-discover=false"]
    }
  }
}
```

#### Direct HTTP Connection (No Federation)

```json
{
  "mcpServers": {
    "jumpboot-remote": {
      "type": "url",
      "url": "http://gpu-server:8080/mcp"
    }
  }
}
```

#### Multiple Direct Connections

```json
{
  "mcpServers": {
    "jumpboot-local": {
      "command": "/path/to/jumpboot-mcp",
      "args": ["-mdns-discover=false"]
    },
    "jumpboot-gpu": {
      "type": "url",
      "url": "http://gpu-server:8080/mcp"
    },
    "jumpboot-data": {
      "type": "url",
      "url": "http://data-server:8080/mcp"
    }
  }
}
```

> **Note:** With direct HTTP connections, tool names are not prefixed. Use federation (stdio with mDNS) if you want automatic prefixing.

### Hybrid Setup: Federation + Direct

```json
{
  "mcpServers": {
    "jumpboot": {
      "command": "/path/to/jumpboot-mcp"
    },
    "jumpboot-cloud": {
      "type": "url",
      "url": "https://cloud-server.example.com:8443/mcp"
    }
  }
}
```

This gives you:
- Local tools from stdio instance
- Auto-discovered LAN servers via mDNS (prefixed names)
- Direct connection to cloud server (unprefixed names)

## MCP Tools Reference

### Environment Management (5 tools)

| Tool | Description |
|------|-------------|
| `create_environment` | Create a new Python environment |
| `list_environments` | List all managed environments |
| `destroy_environment` | Delete an environment and workspace |
| `freeze_environment` | Export environment to JSON |
| `restore_environment` | Recreate from frozen JSON |

### Package Management (3 tools)

| Tool | Description |
|------|-------------|
| `install_packages` | Install packages (pip or conda) |
| `install_requirements` | Install from requirements.txt |
| `list_packages` | List installed packages |

### Code Execution (2 tools)

| Tool | Description |
|------|-------------|
| `run_code` | Execute Python code snippet |
| `run_script` | Execute Python script file |

### REPL Sessions (4 tools)

| Tool | Description |
|------|-------------|
| `repl_create` | Create persistent REPL |
| `repl_execute` | Run code (state preserved) |
| `repl_list` | List active sessions |
| `repl_destroy` | Close session |

### Workspace Management (8 tools)

| Tool | Description |
|------|-------------|
| `workspace_create` | Create code folder |
| `workspace_write_file` | Write file to workspace |
| `workspace_read_file` | Read file from workspace |
| `workspace_list_files` | List workspace files |
| `workspace_delete_file` | Delete file |
| `workspace_run_script` | Run script from workspace |
| `workspace_git_clone` | Clone git repository |
| `workspace_destroy` | Delete workspace |

### Process Management (4 tools)

| Tool | Description |
|------|-------------|
| `spawn_process` | Start background process |
| `list_processes` | List spawned processes |
| `process_output` | Get process stdout/stderr |
| `kill_process` | Terminate process |

## Usage Examples

### Basic Workflow

```
1. create_environment(name="myenv", python_version="3.11") → env_id
2. install_packages(env_id="...", packages=["numpy", "pandas"])
3. run_code(env_id="...", code="import numpy; print(numpy.__version__)")
```

### Using a Remote Server

```
1. gpu-server:create_environment(name="ml-env", python_version="3.11") → env_id
2. gpu-server:install_packages(env_id="...", packages=["torch", "transformers"])
3. gpu-server:run_code(env_id="...", code="import torch; print(torch.cuda.is_available())")
```

### REPL Session

```
1. repl_create(env_id="...", session_name="analysis") → session_id
2. repl_execute(session_id="...", code="x = 42")
3. repl_execute(session_id="...", code="print(x)")  # prints 42
```

### Long-running Process

```
1. workspace_write_file(env_id="...", filename="server.py", content="...")
2. spawn_process(env_id="...", script_path="server.py", capture_output=true)
3. process_output(process_id="...", tail_lines=50)
4. kill_process(process_id="...")
```

## Response Format

All tools return JSON:

```json
{"success": true, "data": {...}, "error": null}
```

Or on error:

```json
{"success": false, "data": null, "error": "descriptive error message"}
```

## Data Storage

All data is stored in `~/.jumpboot-mcp/envs/`:

```
~/.jumpboot-mcp/envs/
├── bases/                    # Cached micromamba bases
│   ├── base_3.11/
│   └── base_3.12/
└── {env-uuid}/              # User environments (venvs)
    ├── bin/
    ├── lib/
    └── workspace/           # Persistent workspace
```

## Dependencies

- [github.com/richinsley/jumpboot](https://github.com/richinsley/jumpboot) - Python environment management
- [github.com/mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) - MCP protocol implementation
- [github.com/hashicorp/mdns](https://github.com/hashicorp/mdns) - mDNS service discovery

## License

MIT
