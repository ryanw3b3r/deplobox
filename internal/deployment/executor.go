package deployment

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"deplobox/internal/security"
	"deplobox/pkg/cmdutil"
	"deplobox/pkg/fileutil"
)

// ExecutionResult represents the result of running a command
type ExecutionResult struct {
	ReturnCode int
	Stdout     string
	Stderr     string
	Duration   time.Duration
}

// OK checks if the execution was successful
func (r *ExecutionResult) OK() bool {
	return r.ReturnCode == 0
}

// Executor handles command execution with timeouts
type Executor struct {
	ProjectRoot string // Root of project (contains shared/, releases/, current)
	executor    *security.SandboxedExecutor
}

// NewExecutor creates a new executor
func NewExecutor(projectRoot string) *Executor {
	return &Executor{
		ProjectRoot: projectRoot,
		executor:    security.NewSandboxedExecutor(projectRoot),
	}
}

// RunCommand executes a command with a timeout in a specific directory
func (e *Executor) RunCommand(ctx context.Context, command []string, timeout int, workingDir string) (*ExecutionResult, error) {
	// Use pkg/cmdutil for command execution
	result, err := cmdutil.Run(
		ctx,
		cmdutil.ExecOptions{
			Dir:            workingDir,
			Timeout:        time.Duration(timeout) * time.Second,
			CombinedOutput: true,
		},
		command,
	)

	execResult := &ExecutionResult{
		Stdout: string(result.Output),
		Stderr: string(result.Output), // CombinedOutput
	}

	if result != nil {
		execResult.ReturnCode = result.ExitCode
		execResult.Duration = result.Duration
	}

	if err != nil {
		return execResult, err
	}

	return execResult, nil
}

// RunCommandSecure executes a command using the sandboxed executor
// This validates that the command is in the allowed list and checks for shell metacharacters
func (e *Executor) RunCommandSecure(ctx context.Context, command []string, timeout int, workingDir string) (*ExecutionResult, error) {
	// Create executor with working directory
	executor := security.NewSandboxedExecutor(workingDir)

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// Execute command
	start := time.Now()
	output, err := executor.Execute(ctx, command)
	duration := time.Since(start)

	execResult := &ExecutionResult{
		Stdout:   string(output),
		Stderr:   string(output), // CombinedOutput
		Duration: duration,
	}

	if err != nil {
		return execResult, err
	}

	return execResult, nil
}

// ParseCommand converts a command from string or []interface{} to []string
func ParseCommand(cmd interface{}) ([]string, error) {
	// Use pkg/cmdutil for command parsing
	return cmdutil.ParseCommandList(cmd)
}

// CreateRelease creates a new timestamped release directory
func (e *Executor) CreateRelease(ctx context.Context, branch string, timeout int) (string, *ExecutionResult, error) {
	// Validate branch name
	if err := security.ValidateBranchName(branch); err != nil {
		return "", nil, fmt.Errorf("invalid branch name: %w", err)
	}

	releasesDir := filepath.Join(e.ProjectRoot, "releases")

	// Generate timestamp for new release
	timestamp := time.Now().Format("2006-01-02-15-04-05")
	releaseDir := filepath.Join(releasesDir, timestamp)

	// Check if current exists
	currentLink := filepath.Join(e.ProjectRoot, "current")
	if !fileutil.SymlinkExists(currentLink) {
		// No current release, need to use git pull from a remote
		// This case should only happen if installer didn't run properly
		return "", nil, fmt.Errorf("no current release found - initial deployment must be done via installer")
	}

	// Get the actual path that current points to
	currentPath, err := fileutil.ResolveSymlink(currentLink)
	if err != nil {
		return "", nil, fmt.Errorf("failed to resolve current symlink: %w", err)
	}

	// Validate paths to prevent path traversal
	if _, err := security.SanitizePathForSymlink(e.ProjectRoot, currentPath); err != nil {
		return "", nil, fmt.Errorf("current symlink points outside project root: %w", err)
	}

	// Create new release by cloning from current
	result, err := e.RunCommand(ctx, []string{"cp", "-a", currentPath, releaseDir}, timeout, e.ProjectRoot)
	if err != nil || !result.OK() {
		return "", result, fmt.Errorf("failed to create release copy: %w", err)
	}

	return releaseDir, result, nil
}

// RunGitPull executes git pull in the release directory
func (e *Executor) RunGitPull(ctx context.Context, releaseDir, branch string, timeout int) (*ExecutionResult, error) {
	// Validate branch name
	if err := security.ValidateBranchName(branch); err != nil {
		return nil, fmt.Errorf("invalid branch name: %w", err)
	}

	// Validate release directory path
	if _, err := security.SanitizePathForSymlink(e.ProjectRoot, releaseDir); err != nil {
		return nil, fmt.Errorf("release directory outside project root: %w", err)
	}

	// Reset any local changes
	resetCmd := []string{"git", "reset", "--hard", "HEAD"}
	if result, err := e.RunCommand(ctx, resetCmd, timeout, releaseDir); err != nil || !result.OK() {
		return result, fmt.Errorf("git reset failed: %w", err)
	}

	// Pull latest changes
	cmd := []string{"git", "pull", "origin", branch}
	return e.RunCommand(ctx, cmd, timeout, releaseDir)
}

// CopySharedFiles copies files from shared directory to release
func (e *Executor) CopySharedFiles(ctx context.Context, releaseDir string, timeout int) (*ExecutionResult, error) {
	sharedDir := filepath.Join(e.ProjectRoot, "shared")

	// Validate paths to prevent path traversal
	if _, err := security.SanitizePathForSymlink(e.ProjectRoot, sharedDir); err != nil {
		return nil, fmt.Errorf("shared directory outside project root: %w", err)
	}
	if _, err := security.SanitizePathForSymlink(e.ProjectRoot, releaseDir); err != nil {
		return nil, fmt.Errorf("release directory outside project root: %w", err)
	}

	// Check if shared directory exists and has contents
	if !fileutil.DirExists(sharedDir) {
		// No shared directory
		return &ExecutionResult{ReturnCode: 0}, nil
	}

	entries, err := os.ReadDir(sharedDir)
	if err != nil || len(entries) == 0 {
		// No shared files to copy
		return &ExecutionResult{ReturnCode: 0}, nil
	}

	// Use rsync to copy/merge shared files
	// Note: Paths are validated above, but rsync itself is in allowed commands
	cmd := []string{"rsync", "-a", sharedDir + "/", releaseDir + "/"}
	return e.RunCommand(ctx, cmd, timeout, e.ProjectRoot)
}

// RunPostDeployCommands executes all post-deploy commands sequentially in the release directory
func (e *Executor) RunPostDeployCommands(ctx context.Context, releaseDir string, commands []interface{}, timeout int) ([]*ExecutionResult, error) {
	results := make([]*ExecutionResult, 0, len(commands))

	// Validate release directory path
	if _, err := security.SanitizePathForSymlink(e.ProjectRoot, releaseDir); err != nil {
		return results, fmt.Errorf("release directory outside project root: %w", err)
	}

	for i, cmdInterface := range commands {
		// Parse command using pkg/cmdutil
		cmd, err := ParseCommand(cmdInterface)
		if err != nil {
			return results, fmt.Errorf("failed to parse post_deploy command %d: %w", i, err)
		}

		// Run command in release directory
		// Using RunCommand (not RunCommandSecure) to allow all post-deploy commands
		// Security validation happens at config load time
		result, err := e.RunCommand(ctx, cmd, timeout, releaseDir)
		results = append(results, result)

		if err != nil {
			return results, fmt.Errorf("post_deploy command %d failed: %w (command: %s)",
				i, err, cmdutil.FormatCommand(cmd))
		}

		if !result.OK() {
			return results, fmt.Errorf("post_deploy command %d exited with code %d (command: %s)",
				i, result.ReturnCode, cmdutil.FormatCommand(cmd))
		}
	}

	return results, nil
}

// RunPostActivateCommands executes all post-activate commands sequentially in the current directory
// These commands run after the deployment has been activated (current symlink updated)
func (e *Executor) RunPostActivateCommands(ctx context.Context, commands []interface{}, timeout int) ([]*ExecutionResult, error) {
	results := make([]*ExecutionResult, 0, len(commands))

	// Run commands in the current directory (which now points to the new release)
	currentDir := filepath.Join(e.ProjectRoot, "current")

	for i, cmdInterface := range commands {
		// Parse command using pkg/cmdutil
		cmd, err := ParseCommand(cmdInterface)
		if err != nil {
			return results, fmt.Errorf("failed to parse post_activate command %d: %w", i, err)
		}

		// Run command in current directory
		// Using RunCommand (not RunCommandSecure) to allow all post-activate commands
		// Security validation happens at config load time
		result, err := e.RunCommand(ctx, cmd, timeout, currentDir)
		results = append(results, result)

		if err != nil {
			return results, fmt.Errorf("post_activate command %d failed: %w (command: %s)",
				i, err, cmdutil.FormatCommand(cmd))
		}

		if !result.OK() {
			return results, fmt.Errorf("post_activate command %d exited with code %d (command: %s)",
				i, result.ReturnCode, cmdutil.FormatCommand(cmd))
		}
	}

	return results, nil
}

// UpdateCurrentSymlink atomically updates the current symlink to point to new release
func (e *Executor) UpdateCurrentSymlink(releaseDir string) error {
	// Validate release directory path
	if _, err := security.SanitizePathForSymlink(e.ProjectRoot, releaseDir); err != nil {
		return fmt.Errorf("release directory outside project root: %w", err)
	}

	currentLink := filepath.Join(e.ProjectRoot, "current")

	// Create relative path for symlink
	relPath, err := filepath.Rel(e.ProjectRoot, releaseDir)
	if err != nil {
		return fmt.Errorf("failed to calculate relative path: %w", err)
	}

	// Use pkg/fileutil for atomic symlink update
	if err := fileutil.UpdateSymlinkAtomic(currentLink, relPath); err != nil {
		return fmt.Errorf("failed to update current symlink: %w", err)
	}

	return nil
}

// CleanupOldReleases removes old releases, keeping the specified number of most recent
func (e *Executor) CleanupOldReleases(keepCount int) error {
	releasesDir := filepath.Join(e.ProjectRoot, "releases")

	entries, err := os.ReadDir(releasesDir)
	if err != nil {
		return fmt.Errorf("failed to read releases directory: %w", err)
	}

	// Filter only directories
	var releases []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() {
			releases = append(releases, entry)
		}
	}

	// Keep only if we have more than keepCount releases
	if len(releases) <= keepCount {
		return nil
	}

	// Sort by name (timestamp) in descending order
	// Since names are timestamps in YYYY-MM-DD-HH-MM-SS format, alphabetical sort works
	for i := 0; i < len(releases)-1; i++ {
		for j := i + 1; j < len(releases); j++ {
			if releases[i].Name() < releases[j].Name() {
				releases[i], releases[j] = releases[j], releases[i]
			}
		}
	}

	// Remove old releases
	for i := keepCount; i < len(releases); i++ {
		releasePath := filepath.Join(releasesDir, releases[i].Name())

		// Validate path before deletion
		if _, err := security.SanitizePathForSymlink(e.ProjectRoot, releasePath); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: skipping deletion of release outside project root: %s\n", releases[i].Name())
			continue
		}

		if err := os.RemoveAll(releasePath); err != nil {
			// Log error but continue
			fmt.Fprintf(os.Stderr, "Warning: failed to remove old release %s: %v\n", releases[i].Name(), err)
		}
	}

	return nil
}

// RestorePreviousRelease switches the current symlink to the previous release
func (e *Executor) RestorePreviousRelease() (string, string, error) {
	currentLink := filepath.Join(e.ProjectRoot, "current")
	releasesDir := filepath.Join(e.ProjectRoot, "releases")

	// Check if current symlink exists
	if !fileutil.SymlinkExists(currentLink) {
		return "", "", fmt.Errorf("no current release found (current symlink missing)")
	}

	// Get the current release path
	currentPath, err := fileutil.ResolveSymlink(currentLink)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve current symlink: %w", err)
	}

	// Validate current path
	if _, err := security.SanitizePathForSymlink(e.ProjectRoot, currentPath); err != nil {
		return "", "", fmt.Errorf("current symlink points outside project root: %w", err)
	}

	// Get current release name (basename)
	currentReleaseName := filepath.Base(currentPath)

	// Read all releases
	entries, err := os.ReadDir(releasesDir)
	if err != nil {
		return "", "", fmt.Errorf("failed to read releases directory: %w", err)
	}

	// Filter only directories and collect release names
	var releaseNames []string
	for _, entry := range entries {
		if entry.IsDir() {
			releaseNames = append(releaseNames, entry.Name())
		}
	}

	// Need at least 2 releases to restore
	if len(releaseNames) < 2 {
		return "", "", fmt.Errorf("cannot restore: only one release exists (need at least 2 releases)")
	}

	// Sort releases by name (timestamp) in descending order (newest first)
	for i := 0; i < len(releaseNames)-1; i++ {
		for j := i + 1; j < len(releaseNames); j++ {
			if releaseNames[i] < releaseNames[j] {
				releaseNames[i], releaseNames[j] = releaseNames[j], releaseNames[i]
			}
		}
	}

	// Find the current release in the sorted list
	currentIndex := -1
	for i, name := range releaseNames {
		if name == currentReleaseName {
			currentIndex = i
			break
		}
	}

	if currentIndex == -1 {
		return "", "", fmt.Errorf("current release '%s' not found in releases directory", currentReleaseName)
	}

	// Check if there's a previous release
	if currentIndex >= len(releaseNames)-1 {
		return "", "", fmt.Errorf("cannot restore: current release '%s' is already the oldest", currentReleaseName)
	}

	// Get the previous release (next in the sorted list)
	previousReleaseName := releaseNames[currentIndex+1]
	previousReleasePath := filepath.Join(releasesDir, previousReleaseName)

	// Validate previous release path exists
	if !fileutil.DirExists(previousReleasePath) {
		return "", "", fmt.Errorf("previous release directory does not exist: %s", previousReleasePath)
	}

	// Validate previous release path
	if _, err := security.SanitizePathForSymlink(e.ProjectRoot, previousReleasePath); err != nil {
		return "", "", fmt.Errorf("previous release directory outside project root: %w", err)
	}

	// Update the current symlink to point to previous release
	if err := e.UpdateCurrentSymlink(previousReleasePath); err != nil {
		return "", "", fmt.Errorf("failed to update current symlink: %w", err)
	}

	return currentReleaseName, previousReleaseName, nil
}
