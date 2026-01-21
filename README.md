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

## Configuration

### Claude Desktop

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

### Claude Code

Add to your MCP settings:

```json
{
  "mcpServers": {
    "jumpboot": {
      "command": "/path/to/jumpboot-mcp"
    }
  }
}
```

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
