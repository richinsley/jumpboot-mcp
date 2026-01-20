# Jumpboot MCP Server

An MCP (Model Context Protocol) server that wraps the [Jumpboot](https://github.com/richinsley/jumpboot) Go library, enabling AI assistants to dynamically create and manage Python environments, install packages, and execute Python code.

## Features

- **Environment Management**: Create isolated Python environments using micromamba or venv
- **Package Installation**: Install packages via pip or conda
- **Code Execution**: Run Python code snippets or script files
- **REPL Sessions**: Maintain persistent Python REPL sessions with preserved state
- **Workspace Management**: Create temp folders for code, write files, and execute scripts
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

## MCP Tools (21 total)

### Environment Management

| Tool | Description |
|------|-------------|
| `create_environment` | Create a new Python environment (micromamba or venv) |
| `list_environments` | List all managed environments |
| `destroy_environment` | Delete an environment and its workspace |
| `freeze_environment` | Export environment to JSON |
| `restore_environment` | Recreate environment from frozen JSON |

### Package Management

| Tool | Description |
|------|-------------|
| `install_packages` | Install Python packages (pip or conda) |
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

### Workspace (Temp Code Folders)

| Tool | Description |
|------|-------------|
| `workspace_create` | Create a temp code folder for an environment |
| `workspace_write_file` | Write a file to the workspace (supports subdirs) |
| `workspace_read_file` | Read a file from the workspace (supports subdirs) |
| `workspace_list_files` | List files in the workspace or subdirectory |
| `workspace_delete_file` | Delete a file from the workspace |
| `workspace_run_script` | Run a script from the workspace (supports subdirs) |
| `workspace_git_clone` | Clone a git repository into the workspace |
| `workspace_destroy` | Delete the workspace |

## Usage Examples

### Basic Workflow

1. **Create an environment**:
   ```
   create_environment(name="myenv", python_version="3.11", use_micromamba=true)
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

### Git Clone Workflow

1. **Create workspace and clone repo**:
   ```
   workspace_create(env_id="...")
   workspace_git_clone(env_id="...", repo_url="https://github.com/user/repo.git")
   → Returns clone path and directory name
   ```

2. **List files in cloned repo**:
   ```
   workspace_list_files(env_id="...", path="repo/src")
   ```

3. **Run a script from the repo**:
   ```
   workspace_run_script(env_id="...", filename="repo/main.py")
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

- Environments: `~/.jumpboot-mcp/envs/<env_id>/`
- Workspaces: System temp directory (`/tmp/jumpboot-workspace-<env_id>-*/`)

## Dependencies

- [github.com/richinsley/jumpboot](https://github.com/richinsley/jumpboot) - Python environment management
- [github.com/mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) - MCP protocol implementation

## License

MIT
