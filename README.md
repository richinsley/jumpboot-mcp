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

## Usage

### Transport Options

The server supports two transport modes:

#### stdio (default)
Standard input/output transport for local use with Claude Desktop or Claude Code:
```bash
./jumpboot-mcp
```

#### HTTP (Streamable HTTP with SSE)
HTTP transport for containerized deployments or remote access:
```bash
./jumpboot-mcp -transport http -addr :8080 -endpoint /mcp
```

**HTTP Options:**
| Flag | Default | Description |
|------|---------|-------------|
| `-transport` | `stdio` | Transport type: `stdio` or `http` |
| `-addr` | `:8080` | HTTP server address |
| `-endpoint` | `/mcp` | HTTP endpoint path |
| `-stateless` | `false` | Run in stateless mode (no session tracking) |
| `-tls-cert` | | TLS certificate file (enables HTTPS) |
| `-tls-key` | | TLS key file (enables HTTPS) |

**HTTPS Example:**
```bash
./jumpboot-mcp -transport http -addr :8443 -tls-cert cert.pem -tls-key key.pem
```

## Configuration

### Claude Desktop

Add to `~/.config/claude/claude_desktop_config.json`:

**stdio (local binary):**
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

> **Note:** Claude Desktop currently only supports stdio transport for local binaries. For HTTP transport, use Claude Code or another MCP client with HTTP support.

### Claude Code

Add to your MCP settings (`.mcp.json` or via `claude mcp add`):

**stdio (local binary):**
```json
{
  "mcpServers": {
    "jumpboot": {
      "command": "/path/to/jumpboot-mcp"
    }
  }
}
```

**HTTP (remote/container):**
```json
{
  "mcpServers": {
    "jumpboot": {
      "type": "url",
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

**HTTPS (remote/container with TLS):**
```json
{
  "mcpServers": {
    "jumpboot": {
      "type": "url",
      "url": "https://your-server:8443/mcp"
    }
  }
}
```

### Docker

A Dockerfile is included for containerized deployments using HTTP transport.

#### Build the Image

```bash
docker build -t jumpboot-mcp .
```

#### Run the Container

Basic usage:
```bash
docker run -p 8080:8080 jumpboot-mcp
```

With persistent storage (recommended):
```bash
docker run -p 8080:8080 -v jumpboot-data:/root/.jumpboot-mcp jumpboot-mcp
```

The volume persists cached micromamba bases and environments across container restarts, avoiding repeated Python downloads.

#### Custom Configuration

Override the default command to change options:
```bash
# Different port
docker run -p 9000:9000 jumpboot-mcp -transport http -addr :9000

# Custom endpoint
docker run -p 8080:8080 jumpboot-mcp -transport http -addr :8080 -endpoint /api/mcp

# Stateless mode
docker run -p 8080:8080 jumpboot-mcp -transport http -addr :8080 -stateless
```

#### HTTPS Configuration

To enable HTTPS, mount your TLS certificate and key into the container:

```bash
docker run -p 8443:8443 \
  -v /path/to/cert.pem:/certs/cert.pem:ro \
  -v /path/to/key.pem:/certs/key.pem:ro \
  -v jumpboot-data:/root/.jumpboot-mcp \
  jumpboot-mcp \
  -transport http -addr :8443 -tls-cert /certs/cert.pem -tls-key /certs/key.pem
```

Or mount a directory containing both files:

```bash
docker run -p 8443:8443 \
  -v /path/to/certs:/certs:ro \
  -v jumpboot-data:/root/.jumpboot-mcp \
  jumpboot-mcp \
  -transport http -addr :8443 -tls-cert /certs/cert.pem -tls-key /certs/key.pem
```

#### Docker Compose Example

HTTP:
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

HTTPS:
```yaml
version: '3.8'
services:
  jumpboot-mcp:
    build: .
    ports:
      - "8443:8443"
    volumes:
      - jumpboot-data:/root/.jumpboot-mcp
      - ./certs:/certs:ro
    command: ["-transport", "http", "-addr", ":8443", "-tls-cert", "/certs/cert.pem", "-tls-key", "/certs/key.pem"]
    restart: unless-stopped

volumes:
  jumpboot-data:
```

#### Connecting to the Container

HTTP:
```
http://localhost:8080/mcp
```

HTTPS:
```
https://localhost:8443/mcp
```

For MCP clients that support HTTP transport, configure the appropriate URL based on your setup.

## MCP Tools (26 total)

### Environment Management

| Tool | Description |
|------|-------------|
| `create_environment` | Create a new Python environment (venv from cached micromamba base) |
| `list_environments` | List all managed environments |
| `destroy_environment` | Delete an environment and its workspace |
| `freeze_environment` | Export environment to JSON |
| `restore_environment` | Recreate environment from frozen JSON |

### Package Management

| Tool | Description |
|------|-------------|
| `install_packages` | Install Python packages (pip or conda) |
| `install_requirements` | Install packages from a requirements.txt file |
| `list_packages` | List installed packages |

### Code Execution

| Tool | Description |
|------|-------------|
| `run_code` | Execute a Python code snippet |
| `run_script` | Execute a Python script file |

### REPL Sessions

| Tool | Description |
|------|-------------|
| `repl_create` | Create a persistent REPL session |
| `repl_execute` | Run code in REPL (state preserved) |
| `repl_list` | List active REPL sessions |
| `repl_destroy` | Close a REPL session |

### Workspace Management

| Tool | Description |
|------|-------------|
| `workspace_create` | Create a code folder for an environment |
| `workspace_write_file` | Write a file to the workspace (supports subdirs) |
| `workspace_read_file` | Read a file from the workspace (supports subdirs) |
| `workspace_list_files` | List files in the workspace or subdirectory |
| `workspace_delete_file` | Delete a file from the workspace |
| `workspace_run_script` | Run a script from the workspace (supports subdirs) |
| `workspace_git_clone` | Clone a git repository into the workspace |
| `workspace_destroy` | Delete the workspace |

### Process Management (Long-running)

| Tool | Description |
|------|-------------|
| `spawn_process` | Start a Python script in the background (GUI apps, servers, games) |
| `list_processes` | List all spawned processes |
| `process_output` | Get stdout/stderr from a spawned process |
| `kill_process` | Terminate a spawned process |

## Usage Examples

### Basic Workflow

1. **Create an environment**:
   ```
   create_environment(name="myenv", python_version="3.11")
   → Returns env_id
   ```

2. **Install packages**:
   ```
   install_packages(env_id="...", packages=["numpy", "pandas"])
   ```

3. **Run code**:
   ```
   run_code(env_id="...", code="import numpy; print(numpy.__version__)")
   ```

### Git Clone Workflow

1. **Create workspace and clone repo**:
   ```
   workspace_create(env_id="...")
   workspace_git_clone(env_id="...", repo_url="https://github.com/user/repo.git")
   → Returns clone path and directory name
   ```

2. **Install dependencies from requirements.txt**:
   ```
   install_requirements(env_id="...", requirements_path="repo/requirements.txt")
   ```

3. **Run a script from the repo**:
   ```
   workspace_run_script(env_id="...", filename="repo/main.py")
   ```

### Workspace Workflow

1. **Create workspace**:
   ```
   workspace_create(env_id="...")
   → Returns workspace path
   ```

2. **Write a script**:
   ```
   workspace_write_file(env_id="...", filename="analysis.py", content="...")
   ```

3. **Run the script**:
   ```
   workspace_run_script(env_id="...", filename="analysis.py")
   ```

### REPL Session

1. **Create REPL**:
   ```
   repl_create(env_id="...", session_name="data_analysis")
   → Returns session_id
   ```

2. **Execute code (state preserved)**:
   ```
   repl_execute(session_id="...", code="x = 42")
   repl_execute(session_id="...", code="print(x)")  # prints 42
   ```

### Long-running Process (GUI App/Server/Game)

1. **Write a GUI script**:
   ```
   workspace_write_file(env_id="...", filename="game.py", content="...")
   ```

2. **Spawn the process** (runs in background):
   ```
   spawn_process(env_id="...", script_path="game.py", capture_output=false)
   → Returns process_id, PID
   ```

3. **Check running processes**:
   ```
   list_processes()
   → Returns list of {process_id, name, running, pid}
   ```

4. **Terminate when done**:
   ```
   kill_process(process_id="...")
   ```

For servers with logging:
```
spawn_process(env_id="...", script_path="server.py", capture_output=true)
process_output(process_id="...", tail_lines=50)  # Get last 50 lines of logs
```

## Response Format

All tools return JSON responses:

```json
{
  "success": true,
  "data": { ... },
  "error": null
}
```

Or on error:

```json
{
  "success": false,
  "data": null,
  "error": "descriptive error message"
}
```

## Data Storage

All data is stored in `~/.jumpboot-mcp/envs/`:

```
~/.jumpboot-mcp/envs/
├── bases/                    # Cached micromamba base environments
│   ├── base_3.11/           # Base for Python 3.11
│   └── base_3.12/           # Base for Python 3.12
└── {env-uuid}/              # User environments (venvs)
    ├── bin/
    ├── lib/
    ├── pyvenv.cfg
    └── workspace/           # Persistent workspace for this environment
        └── (cloned repos, scripts, etc.)
```

**Environment Creation Strategy**:
- First environment for a Python version creates a micromamba base (slower, downloads Python)
- Subsequent environments reuse the cached base via fast venv creation
- All environments are completely independent of system Python

## Dependencies

- [github.com/richinsley/jumpboot](https://github.com/richinsley/jumpboot) - Python environment management
- [github.com/mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) - MCP protocol implementation

## License

MIT
