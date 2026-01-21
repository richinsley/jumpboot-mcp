package manager

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/richinsley/jumpboot"
)

// DefaultPythonVersion is used when no version is specified
const DefaultPythonVersion = "3.11"

// Manager tracks active environments and REPL sessions
type Manager struct {
	mu               sync.RWMutex
	environments     map[string]*ManagedEnvironment
	replSessions     map[string]*ManagedREPL
	spawnedProcesses map[string]*ManagedProcess
	baseEnvironments map[string]*jumpboot.Environment // base envs by version (e.g., "3.11" -> env)
	baseMu           sync.Mutex                       // separate lock for base environment creation
	baseDir          string
}

// ManagedEnvironment wraps a jumpboot Environment with metadata
type ManagedEnvironment struct {
	ID           string                `json:"id"`
	Name         string                `json:"name"`
	Env          *jumpboot.Environment `json:"-"`
	PythonVer    string                `json:"python_version"`
	WorkspaceDir string                `json:"workspace_dir,omitempty"`
	RootDir      string                `json:"root_dir"` // The venv directory
}

// ManagedREPL wraps a jumpboot REPL session with metadata
type ManagedREPL struct {
	ID        string                      `json:"id"`
	Name      string                      `json:"name"`
	EnvID     string                      `json:"env_id"`
	REPL      *jumpboot.REPLPythonProcess `json:"-"`
}

// EnvironmentInfo is the serializable info about an environment
type EnvironmentInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	PythonVersion string `json:"python_version"`
	EnvPath       string `json:"env_path"`
	WorkspaceDir  string `json:"workspace_dir,omitempty"`
}

// REPLInfo is the serializable info about a REPL session
type REPLInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	EnvID string `json:"env_id"`
}

// ManagedProcess wraps a spawned Python process with metadata
type ManagedProcess struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	EnvID         string       `json:"env_id"`
	Cmd           *exec.Cmd    `json:"-"`
	StartTime     time.Time    `json:"start_time"`
	CaptureOutput bool         `json:"capture_output"`
	outputMu      sync.RWMutex // protects outputLines
	outputLines   []string     // circular buffer of output lines
	maxLines      int          // max lines to keep
	done          chan struct{}
	exitCode      int
	exited        bool
}

// ProcessInfo is the serializable info about a spawned process
type ProcessInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	EnvID     string    `json:"env_id"`
	PID       int       `json:"pid"`
	StartTime time.Time `json:"start_time"`
	Running   bool      `json:"running"`
	ExitCode  int       `json:"exit_code,omitempty"`
}

// NewManager creates a new environment manager
func NewManager(baseDir string) (*Manager, error) {
	if baseDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		baseDir = filepath.Join(home, ".jumpboot-mcp", "envs")
	}

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	return &Manager{
		environments:     make(map[string]*ManagedEnvironment),
		replSessions:     make(map[string]*ManagedREPL),
		spawnedProcesses: make(map[string]*ManagedProcess),
		baseEnvironments: make(map[string]*jumpboot.Environment),
		baseDir:          baseDir,
	}, nil
}

// getOrCreateBase returns a base micromamba environment for the given Python version.
// If one doesn't exist, it creates it. Uses baseMu to serialize base creation.
func (m *Manager) getOrCreateBase(pythonVersion string) (*jumpboot.Environment, error) {
	// Quick check with read lock
	m.mu.RLock()
	if baseEnv, ok := m.baseEnvironments[pythonVersion]; ok {
		m.mu.RUnlock()
		return baseEnv, nil
	}
	m.mu.RUnlock()

	// Serialize base creation (but don't block other operations)
	m.baseMu.Lock()
	defer m.baseMu.Unlock()

	// Double-check after acquiring lock
	m.mu.RLock()
	if baseEnv, ok := m.baseEnvironments[pythonVersion]; ok {
		m.mu.RUnlock()
		return baseEnv, nil
	}
	m.mu.RUnlock()

	// Create a new base environment (slow operation, no locks held)
	baseName := fmt.Sprintf("base_%s", pythonVersion)
	basePath := filepath.Join(m.baseDir, "bases", baseName)

	baseEnv, err := jumpboot.CreateEnvironmentMamba(baseName, basePath, pythonVersion, "conda-forge", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create base environment for Python %s: %w", pythonVersion, err)
	}

	// Store the base environment
	m.mu.Lock()
	m.baseEnvironments[pythonVersion] = baseEnv
	m.mu.Unlock()

	return baseEnv, nil
}

// CreateEnvironment creates a new Python environment.
// Creates a venv from a cached micromamba base environment (independent of system Python).
func (m *Manager) CreateEnvironment(name, pythonVersion string) (*EnvironmentInfo, error) {
	// Use default version if not specified
	if pythonVersion == "" {
		pythonVersion = DefaultPythonVersion
	}

	// Generate ID and path for the venv
	id := uuid.New().String()
	envPath := filepath.Join(m.baseDir, id)

	// Get or create base environment (handles its own locking)
	baseEnv, err := m.getOrCreateBase(pythonVersion)
	if err != nil {
		return nil, err
	}

	// Create venv from base (runs without holding the main lock)
	env, err := jumpboot.CreateVenvEnvironment(baseEnv, envPath, jumpboot.VenvOptions{}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create venv: %w", err)
	}

	managed := &ManagedEnvironment{
		ID:        id,
		Name:      name,
		Env:       env,
		PythonVer: pythonVersion,
		RootDir:   envPath,
	}

	// Only hold lock briefly to store the result
	m.mu.Lock()
	m.environments[id] = managed
	m.mu.Unlock()

	return &EnvironmentInfo{
		ID:            id,
		Name:          name,
		PythonVersion: env.PythonVersion.String(),
		EnvPath:       env.EnvPath,
	}, nil
}

// GetEnvironment retrieves an environment by ID
func (m *Manager) GetEnvironment(id string) (*ManagedEnvironment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	env, ok := m.environments[id]
	if !ok {
		return nil, fmt.Errorf("environment not found: %s", id)
	}
	return env, nil
}

// ListEnvironments returns info about all managed environments
func (m *Manager) ListEnvironments() []EnvironmentInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]EnvironmentInfo, 0, len(m.environments))
	for _, env := range m.environments {
		result = append(result, EnvironmentInfo{
			ID:            env.ID,
			Name:          env.Name,
			PythonVersion: env.Env.PythonVersion.String(),
			EnvPath:       env.Env.EnvPath,
			WorkspaceDir:  env.WorkspaceDir,
		})
	}
	return result
}

// DestroyEnvironment removes an environment and cleans up its resources
func (m *Manager) DestroyEnvironment(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	env, ok := m.environments[id]
	if !ok {
		return fmt.Errorf("environment not found: %s", id)
	}

	// Kill any spawned processes using this environment
	for procID, proc := range m.spawnedProcesses {
		if proc.EnvID == id {
			proc.outputMu.RLock()
			exited := proc.exited
			proc.outputMu.RUnlock()

			if !exited && proc.Cmd.Process != nil {
				proc.Cmd.Process.Kill()
				<-proc.done
			}
			delete(m.spawnedProcesses, procID)
		}
	}

	// Close any REPL sessions using this environment
	for replID, repl := range m.replSessions {
		if repl.EnvID == id {
			if repl.REPL != nil {
				repl.REPL.Close()
			}
			delete(m.replSessions, replID)
		}
	}

	// Remove the workspace directory if it exists
	if env.WorkspaceDir != "" {
		os.RemoveAll(env.WorkspaceDir)
	}

	// Remove the root environment directory (contains bin, envs, pkgs)
	if env.RootDir != "" {
		if err := os.RemoveAll(env.RootDir); err != nil {
			return fmt.Errorf("failed to remove environment directory: %w", err)
		}
	}

	delete(m.environments, id)
	return nil
}

// FreezeEnvironment exports an environment to JSON
func (m *Manager) FreezeEnvironment(id string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	env, ok := m.environments[id]
	if !ok {
		return "", fmt.Errorf("environment not found: %s", id)
	}

	// Create a temp file for the freeze
	tmpFile, err := os.CreateTemp("", "freeze-*.json")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	if err := env.Env.FreezeToFile(tmpPath); err != nil {
		return "", fmt.Errorf("failed to freeze environment: %w", err)
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to read frozen environment: %w", err)
	}

	return string(data), nil
}

// RestoreEnvironment recreates an environment from frozen JSON
func (m *Manager) RestoreEnvironment(name, frozenJSON string) (*EnvironmentInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := uuid.New().String()
	envPath := filepath.Join(m.baseDir, id)

	// Write the frozen JSON to a temp file
	tmpFile, err := os.CreateTemp("", "restore-*.json")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(frozenJSON); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("failed to write frozen JSON: %w", err)
	}
	tmpFile.Close()

	env, err := jumpboot.CreateEnvironmentFromJSONFile(tmpPath, envPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to restore environment: %w", err)
	}

	managed := &ManagedEnvironment{
		ID:        id,
		Name:      name,
		Env:       env,
		PythonVer: env.PythonVersion.String(),
		RootDir:   envPath,
	}

	m.environments[id] = managed

	return &EnvironmentInfo{
		ID:            id,
		Name:          name,
		PythonVersion: env.PythonVersion.String(),
		EnvPath:       env.EnvPath,
	}, nil
}

// CreateREPL creates a new REPL session for an environment
func (m *Manager) CreateREPL(envID, sessionName string) (*REPLInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	env, ok := m.environments[envID]
	if !ok {
		return nil, fmt.Errorf("environment not found: %s", envID)
	}

	repl, err := env.Env.NewREPLPythonProcess(nil, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create REPL: %w", err)
	}

	id := uuid.New().String()
	managed := &ManagedREPL{
		ID:    id,
		Name:  sessionName,
		EnvID: envID,
		REPL:  repl,
	}

	m.replSessions[id] = managed

	return &REPLInfo{
		ID:    id,
		Name:  sessionName,
		EnvID: envID,
	}, nil
}

// GetREPL retrieves a REPL session by ID
func (m *Manager) GetREPL(id string) (*ManagedREPL, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	repl, ok := m.replSessions[id]
	if !ok {
		return nil, fmt.Errorf("REPL session not found: %s", id)
	}
	return repl, nil
}

// ListREPLs returns info about all active REPL sessions
func (m *Manager) ListREPLs() []REPLInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]REPLInfo, 0, len(m.replSessions))
	for _, repl := range m.replSessions {
		result = append(result, REPLInfo{
			ID:    repl.ID,
			Name:  repl.Name,
			EnvID: repl.EnvID,
		})
	}
	return result
}

// DestroyREPL closes and removes a REPL session
func (m *Manager) DestroyREPL(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	repl, ok := m.replSessions[id]
	if !ok {
		return fmt.Errorf("REPL session not found: %s", id)
	}

	if repl.REPL != nil {
		if err := repl.REPL.Close(); err != nil {
			return fmt.Errorf("failed to close REPL: %w", err)
		}
	}

	delete(m.replSessions, id)
	return nil
}

// ExecuteREPL runs code in a REPL session
func (m *Manager) ExecuteREPL(id, code string) (string, error) {
	m.mu.RLock()
	repl, ok := m.replSessions[id]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("REPL session not found: %s", id)
	}

	result, err := repl.REPL.Execute(code, true)
	if err != nil {
		return "", fmt.Errorf("failed to execute code: %w", err)
	}

	return result, nil
}

// Shutdown cleans up all resources
func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Kill all spawned processes
	for _, proc := range m.spawnedProcesses {
		proc.outputMu.RLock()
		exited := proc.exited
		proc.outputMu.RUnlock()

		if !exited && proc.Cmd.Process != nil {
			proc.Cmd.Process.Kill()
			<-proc.done
		}
	}
	m.spawnedProcesses = make(map[string]*ManagedProcess)

	// Close all REPL sessions
	for _, repl := range m.replSessions {
		if repl.REPL != nil {
			repl.REPL.Close()
		}
	}
	m.replSessions = make(map[string]*ManagedREPL)
}

// InstallPackages installs packages in an environment
func (m *Manager) InstallPackages(envID string, packages []string, useConda bool) error {
	m.mu.RLock()
	env, ok := m.environments[envID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("environment not found: %s", envID)
	}

	if useConda {
		for _, pkg := range packages {
			if err := env.Env.MicromambaInstallPackage(pkg, "conda-forge"); err != nil {
				return fmt.Errorf("failed to install %s via conda: %w", pkg, err)
			}
		}
	} else {
		if err := env.Env.PipInstallPackages(packages, "", "", false, nil); err != nil {
			return fmt.Errorf("failed to install packages via pip: %w", err)
		}
	}

	return nil
}

// InstallRequirements installs packages from a requirements.txt file in the workspace
func (m *Manager) InstallRequirements(envID, requirementsPath string, upgrade bool) error {
	m.mu.RLock()
	env, ok := m.environments[envID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("environment not found: %s", envID)
	}

	if env.WorkspaceDir == "" {
		return fmt.Errorf("no workspace created for environment: %s", envID)
	}

	// Sanitize and validate path
	fullPath, err := safeJoinPath(env.WorkspaceDir, requirementsPath)
	if err != nil {
		return err
	}

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return fmt.Errorf("requirements file not found: %s", requirementsPath)
	}

	// Use the environment's pip to install from requirements file
	// Run: python -m pip install -r requirements.txt [--upgrade]
	args := []string{"-m", "pip", "install", "-r", fullPath}
	if upgrade {
		args = append(args, "--upgrade")
	}

	output, err := env.Env.RunPythonReadCombined(args[0], args[1:]...)
	if err != nil {
		return fmt.Errorf("failed to install from requirements: %w\nOutput: %s", err, output)
	}

	return nil
}

// ListPackages returns installed packages in an environment
func (m *Manager) ListPackages(envID string) ([]PackageInfo, error) {
	m.mu.RLock()
	env, ok := m.environments[envID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("environment not found: %s", envID)
	}

	// Use pip freeze to list packages
	output, err := env.Env.RunPythonReadStdout("-m", "pip", "freeze")
	if err != nil {
		return nil, fmt.Errorf("failed to list packages: %w", err)
	}

	var packages []PackageInfo
	lines := splitLines(output)
	for _, line := range lines {
		line = trimSpace(line)
		if line == "" {
			continue
		}
		// Parse "package==version" format
		parts := splitOnce(line, "==")
		pkg := PackageInfo{Name: parts[0]}
		if len(parts) > 1 {
			pkg.Version = parts[1]
		}
		packages = append(packages, pkg)
	}

	return packages, nil
}

// RunCode executes Python code in an environment
func (m *Manager) RunCode(envID, code string, inputJSON string) (string, error) {
	m.mu.RLock()
	env, ok := m.environments[envID]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("environment not found: %s", envID)
	}

	// Create a temporary script file
	tmpFile, err := os.CreateTemp("", "script-*.py")
	if err != nil {
		return "", fmt.Errorf("failed to create temp script: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(code); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to write script: %w", err)
	}
	tmpFile.Close()

	output, err := env.Env.RunPythonReadCombined(tmpPath)
	if err != nil {
		return "", fmt.Errorf("execution failed: %w\nOutput: %s", err, output)
	}

	return output, nil
}

// RunScript executes a Python script file in an environment
func (m *Manager) RunScript(envID, scriptPath string, args []string) (string, error) {
	m.mu.RLock()
	env, ok := m.environments[envID]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("environment not found: %s", envID)
	}

	allArgs := append([]string{scriptPath}, args...)
	output, err := env.Env.RunPythonReadCombined(allArgs[0], allArgs[1:]...)
	if err != nil {
		return "", fmt.Errorf("execution failed: %w\nOutput: %s", err, output)
	}

	return output, nil
}

// PackageInfo describes an installed package
type PackageInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// WorkspaceInfo describes a workspace
type WorkspaceInfo struct {
	EnvID string `json:"env_id"`
	Path  string `json:"path"`
}

// FileInfo describes a file in the workspace
type FileInfo struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size"`
}

// CreateWorkspace creates a code folder for an environment
func (m *Manager) CreateWorkspace(envID string) (*WorkspaceInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	env, ok := m.environments[envID]
	if !ok {
		return nil, fmt.Errorf("environment not found: %s", envID)
	}

	// If workspace already exists, return it
	if env.WorkspaceDir != "" {
		return &WorkspaceInfo{
			EnvID: envID,
			Path:  env.WorkspaceDir,
		}, nil
	}

	// Create workspace inside the environment's directory
	workspaceDir := filepath.Join(env.RootDir, "workspace")
	if err := os.MkdirAll(workspaceDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create workspace: %w", err)
	}

	env.WorkspaceDir = workspaceDir

	return &WorkspaceInfo{
		EnvID: envID,
		Path:  workspaceDir,
	}, nil
}

// GetWorkspace returns the workspace info for an environment
func (m *Manager) GetWorkspace(envID string) (*WorkspaceInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	env, ok := m.environments[envID]
	if !ok {
		return nil, fmt.Errorf("environment not found: %s", envID)
	}

	if env.WorkspaceDir == "" {
		return nil, fmt.Errorf("no workspace created for environment: %s", envID)
	}

	return &WorkspaceInfo{
		EnvID: envID,
		Path:  env.WorkspaceDir,
	}, nil
}

// WriteWorkspaceFile writes a file to the workspace
func (m *Manager) WriteWorkspaceFile(envID, filename, content string) (*FileInfo, error) {
	m.mu.RLock()
	env, ok := m.environments[envID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("environment not found: %s", envID)
	}

	if env.WorkspaceDir == "" {
		return nil, fmt.Errorf("no workspace created for environment: %s", envID)
	}

	// Sanitize and validate path
	filePath, err := safeJoinPath(env.WorkspaceDir, filename)
	if err != nil {
		return nil, err
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	return &FileInfo{
		Name:  filename,
		Path:  filePath,
		IsDir: false,
		Size:  info.Size(),
	}, nil
}

// ReadWorkspaceFile reads a file from the workspace
func (m *Manager) ReadWorkspaceFile(envID, filename string) (string, error) {
	m.mu.RLock()
	env, ok := m.environments[envID]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("environment not found: %s", envID)
	}

	if env.WorkspaceDir == "" {
		return "", fmt.Errorf("no workspace created for environment: %s", envID)
	}

	// Sanitize and validate path
	filePath, err := safeJoinPath(env.WorkspaceDir, filename)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return string(data), nil
}

// ListWorkspaceFiles lists files in the workspace or a subdirectory
func (m *Manager) ListWorkspaceFiles(envID string, subpath string) ([]FileInfo, error) {
	m.mu.RLock()
	env, ok := m.environments[envID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("environment not found: %s", envID)
	}

	if env.WorkspaceDir == "" {
		return nil, fmt.Errorf("no workspace created for environment: %s", envID)
	}

	listDir := env.WorkspaceDir
	if subpath != "" {
		var err error
		listDir, err = safeJoinPath(env.WorkspaceDir, subpath)
		if err != nil {
			return nil, err
		}
	}

	entries, err := os.ReadDir(listDir)
	if err != nil {
		return nil, fmt.Errorf("failed to list workspace: %w", err)
	}

	var files []FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		relPath := entry.Name()
		if subpath != "" {
			relPath = filepath.Join(subpath, entry.Name())
		}
		files = append(files, FileInfo{
			Name:  entry.Name(),
			Path:  relPath,
			IsDir: entry.IsDir(),
			Size:  info.Size(),
		})
	}

	return files, nil
}

// DeleteWorkspaceFile deletes a file from the workspace
func (m *Manager) DeleteWorkspaceFile(envID, filename string) error {
	m.mu.RLock()
	env, ok := m.environments[envID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("environment not found: %s", envID)
	}

	if env.WorkspaceDir == "" {
		return fmt.Errorf("no workspace created for environment: %s", envID)
	}

	// Sanitize and validate path
	filePath, err := safeJoinPath(env.WorkspaceDir, filename)
	if err != nil {
		return err
	}

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// RunWorkspaceScript runs a script from the workspace
func (m *Manager) RunWorkspaceScript(envID, filename string, args []string) (string, error) {
	m.mu.RLock()
	env, ok := m.environments[envID]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("environment not found: %s", envID)
	}

	if env.WorkspaceDir == "" {
		return "", fmt.Errorf("no workspace created for environment: %s", envID)
	}

	// Sanitize and validate path
	scriptPath, err := safeJoinPath(env.WorkspaceDir, filename)
	if err != nil {
		return "", err
	}

	// Check if file exists
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		return "", fmt.Errorf("script not found: %s", filename)
	}

	allArgs := append([]string{scriptPath}, args...)
	output, err := env.Env.RunPythonReadCombined(allArgs[0], allArgs[1:]...)
	if err != nil {
		return "", fmt.Errorf("execution failed: %w\nOutput: %s", err, output)
	}

	return output, nil
}

// DestroyWorkspace removes the workspace directory
func (m *Manager) DestroyWorkspace(envID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	env, ok := m.environments[envID]
	if !ok {
		return fmt.Errorf("environment not found: %s", envID)
	}

	if env.WorkspaceDir == "" {
		return fmt.Errorf("no workspace to destroy for environment: %s", envID)
	}

	if err := os.RemoveAll(env.WorkspaceDir); err != nil {
		return fmt.Errorf("failed to remove workspace: %w", err)
	}

	env.WorkspaceDir = ""
	return nil
}

// GitCloneInfo describes the result of a git clone operation
type GitCloneInfo struct {
	RepoURL   string `json:"repo_url"`
	ClonePath string `json:"clone_path"`
	DirName   string `json:"dir_name"`
}

// GitCloneToWorkspace clones a git repository into the workspace
func (m *Manager) GitCloneToWorkspace(envID, repoURL, dirName string) (*GitCloneInfo, error) {
	m.mu.RLock()
	env, ok := m.environments[envID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("environment not found: %s", envID)
	}

	if env.WorkspaceDir == "" {
		return nil, fmt.Errorf("no workspace created for environment: %s", envID)
	}

	// If dirName is empty, extract from repo URL
	if dirName == "" {
		dirName = extractRepoName(repoURL)
	}

	clonePath := filepath.Join(env.WorkspaceDir, dirName)

	// Check if directory already exists
	if _, err := os.Stat(clonePath); err == nil {
		return nil, fmt.Errorf("directory already exists: %s", dirName)
	}

	// Run git clone
	cmd := exec.Command("git", "clone", repoURL, clonePath)
	cmd.Dir = env.WorkspaceDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git clone failed: %w\nOutput: %s", err, string(output))
	}

	return &GitCloneInfo{
		RepoURL:   repoURL,
		ClonePath: clonePath,
		DirName:   dirName,
	}, nil
}

// extractRepoName extracts the repository name from a git URL
func extractRepoName(repoURL string) string {
	// Handle URLs like:
	// https://github.com/user/repo.git
	// git@github.com:user/repo.git
	// https://github.com/user/repo

	name := repoURL

	// Remove trailing .git
	if len(name) > 4 && name[len(name)-4:] == ".git" {
		name = name[:len(name)-4]
	}

	// Find the last path component
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '/' || name[i] == ':' {
			name = name[i+1:]
			break
		}
	}

	if name == "" {
		name = "repo"
	}

	return name
}

// safeJoinPath safely joins a base path with a relative path, preventing path traversal
func safeJoinPath(base, relPath string) (string, error) {
	// Clean the relative path
	relPath = filepath.Clean(relPath)

	// Prevent absolute paths
	if filepath.IsAbs(relPath) {
		return "", fmt.Errorf("absolute paths not allowed: %s", relPath)
	}

	// Join and clean the full path
	fullPath := filepath.Join(base, relPath)
	fullPath = filepath.Clean(fullPath)

	// Ensure the result is still under the base directory
	if !isSubPath(base, fullPath) {
		return "", fmt.Errorf("path traversal not allowed: %s", relPath)
	}

	return fullPath, nil
}

// isSubPath checks if child is under parent directory
func isSubPath(parent, child string) bool {
	parent = filepath.Clean(parent)
	child = filepath.Clean(child)

	// Add trailing separator to parent to ensure proper prefix matching
	if !os.IsPathSeparator(parent[len(parent)-1]) {
		parent = parent + string(filepath.Separator)
	}

	return len(child) >= len(parent) && child[:len(parent)] == parent
}

// Helper functions for string parsing
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r' || s[start] == '\n') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r' || s[end-1] == '\n') {
		end--
	}
	return s[start:end]
}

func splitOnce(s, sep string) []string {
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			return []string{s[:i], s[i+len(sep):]}
		}
	}
	return []string{s}
}

// SpawnProcess starts a Python script that runs in the background
func (m *Manager) SpawnProcess(envID, scriptPath, name string, args []string, captureOutput bool) (*ProcessInfo, error) {
	m.mu.RLock()
	env, ok := m.environments[envID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("environment not found: %s", envID)
	}

	id := uuid.New().String()
	if name == "" {
		name = filepath.Base(scriptPath)
	}

	// Build command using environment's Python
	cmdArgs := append([]string{scriptPath}, args...)
	cmd := exec.Command(env.Env.PythonPath, cmdArgs...)
	cmd.Dir = env.WorkspaceDir

	// Set up environment variables with the Python environment's bin path
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("PATH=%s%c%s", env.Env.EnvBinPath, filepath.ListSeparator, os.Getenv("PATH")))

	managed := &ManagedProcess{
		ID:            id,
		Name:          name,
		EnvID:         envID,
		Cmd:           cmd,
		StartTime:     time.Now(),
		CaptureOutput: captureOutput,
		outputLines:   make([]string, 0),
		maxLines:      1000, // keep last 1000 lines
		done:          make(chan struct{}),
	}

	if captureOutput {
		// Create pipes for stdout and stderr
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
		}
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
		}

		// Start output capture goroutines
		go managed.captureOutput(stdout)
		go managed.captureOutput(stderr)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start process: %w", err)
	}

	// Monitor process in background
	go func() {
		err := cmd.Wait()
		managed.outputMu.Lock()
		managed.exited = true
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				managed.exitCode = exitErr.ExitCode()
			} else {
				managed.exitCode = -1
			}
		} else {
			managed.exitCode = 0
		}
		managed.outputMu.Unlock()
		close(managed.done)
	}()

	m.mu.Lock()
	m.spawnedProcesses[id] = managed
	m.mu.Unlock()

	return &ProcessInfo{
		ID:        id,
		Name:      name,
		EnvID:     envID,
		PID:       cmd.Process.Pid,
		StartTime: managed.StartTime,
		Running:   true,
	}, nil
}

// captureOutput reads from a reader and stores lines in the buffer
func (p *ManagedProcess) captureOutput(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		p.outputMu.Lock()
		p.outputLines = append(p.outputLines, line)
		// Keep only the last maxLines
		if len(p.outputLines) > p.maxLines {
			p.outputLines = p.outputLines[len(p.outputLines)-p.maxLines:]
		}
		p.outputMu.Unlock()
	}
}

// ListProcesses returns info about all spawned processes
func (m *Manager) ListProcesses() []ProcessInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]ProcessInfo, 0, len(m.spawnedProcesses))
	for _, proc := range m.spawnedProcesses {
		proc.outputMu.RLock()
		info := ProcessInfo{
			ID:        proc.ID,
			Name:      proc.Name,
			EnvID:     proc.EnvID,
			PID:       proc.Cmd.Process.Pid,
			StartTime: proc.StartTime,
			Running:   !proc.exited,
			ExitCode:  proc.exitCode,
		}
		proc.outputMu.RUnlock()
		result = append(result, info)
	}
	return result
}

// GetProcessOutput returns the captured output from a spawned process
func (m *Manager) GetProcessOutput(processID string, tailLines int) ([]string, error) {
	m.mu.RLock()
	proc, ok := m.spawnedProcesses[processID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("process not found: %s", processID)
	}

	if !proc.CaptureOutput {
		return nil, fmt.Errorf("output capture not enabled for process: %s", processID)
	}

	proc.outputMu.RLock()
	defer proc.outputMu.RUnlock()

	if tailLines <= 0 || tailLines >= len(proc.outputLines) {
		// Return all lines
		result := make([]string, len(proc.outputLines))
		copy(result, proc.outputLines)
		return result, nil
	}

	// Return last N lines
	start := len(proc.outputLines) - tailLines
	result := make([]string, tailLines)
	copy(result, proc.outputLines[start:])
	return result, nil
}

// KillProcess terminates a spawned process
func (m *Manager) KillProcess(processID string) error {
	m.mu.Lock()
	proc, ok := m.spawnedProcesses[processID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("process not found: %s", processID)
	}
	m.mu.Unlock()

	proc.outputMu.RLock()
	exited := proc.exited
	proc.outputMu.RUnlock()

	if exited {
		// Process already exited, just remove it
		m.mu.Lock()
		delete(m.spawnedProcesses, processID)
		m.mu.Unlock()
		return nil
	}

	// Kill the process
	if err := proc.Cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill process: %w", err)
	}

	// Wait for it to fully exit
	<-proc.done

	m.mu.Lock()
	delete(m.spawnedProcesses, processID)
	m.mu.Unlock()

	return nil
}

// GetProcessInfo returns info about a specific process
func (m *Manager) GetProcessInfo(processID string) (*ProcessInfo, error) {
	m.mu.RLock()
	proc, ok := m.spawnedProcesses[processID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("process not found: %s", processID)
	}

	proc.outputMu.RLock()
	defer proc.outputMu.RUnlock()

	return &ProcessInfo{
		ID:        proc.ID,
		Name:      proc.Name,
		EnvID:     proc.EnvID,
		PID:       proc.Cmd.Process.Pid,
		StartTime: proc.StartTime,
		Running:   !proc.exited,
		ExitCode:  proc.exitCode,
	}, nil
}

// Response is a standard response format for MCP tools
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// SuccessResponse creates a success response
func SuccessResponse(data interface{}) string {
	resp := Response{Success: true, Data: data}
	b, _ := json.Marshal(resp)
	return string(b)
}

// ErrorResponse creates an error response
func ErrorResponse(err error) string {
	resp := Response{Success: false, Error: err.Error()}
	b, _ := json.Marshal(resp)
	return string(b)
}
