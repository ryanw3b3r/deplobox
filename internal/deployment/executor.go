package deployment

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/kballard/go-shellquote"
)

// ExecutionResult represents the result of running a command
type ExecutionResult struct {
	ReturnCode int
	Stdout     string
	Stderr     string
}

// OK checks if the execution was successful
func (r *ExecutionResult) OK() bool {
	return r.ReturnCode == 0
}

// Executor handles command execution with timeouts
type Executor struct {
	WorkingDir string
}

// NewExecutor creates a new executor
func NewExecutor(workingDir string) *Executor {
	return &Executor{WorkingDir: workingDir}
}

// RunCommand executes a command with a timeout
func (e *Executor) RunCommand(ctx context.Context, command []string, timeout int) (*ExecutionResult, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// Create command
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = e.WorkingDir

	// Capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run command
	err := cmd.Run()

	result := &ExecutionResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	// Handle errors
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ReturnCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			return result, fmt.Errorf("command timed out after %d seconds", timeout)
		} else {
			return result, fmt.Errorf("command execution failed: %w", err)
		}
	}

	return result, nil
}

// ParseCommand converts a command from string or []interface{} to []string
func ParseCommand(cmd interface{}) ([]string, error) {
	switch v := cmd.(type) {
	case string:
		// Parse using shell quoting rules
		return shellquote.Split(v)
	case []interface{}:
		// Convert interface slice to string slice
		result := make([]string, len(v))
		for i, item := range v {
			str, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("command list item %d is not a string: %T", i, item)
			}
			result[i] = str
		}
		return result, nil
	default:
		return nil, fmt.Errorf("command must be string or list, got %T", v)
	}
}

// RunGitPull executes git pull origin <branch>
func (e *Executor) RunGitPull(ctx context.Context, branch string, timeout int) (*ExecutionResult, error) {
	cmd := []string{"git", "pull", "origin", branch}
	return e.RunCommand(ctx, cmd, timeout)
}

// RunPostDeployCommands executes all post-deploy commands sequentially
func (e *Executor) RunPostDeployCommands(ctx context.Context, commands []interface{}, timeout int) ([]*ExecutionResult, error) {
	results := make([]*ExecutionResult, 0, len(commands))

	for i, cmdInterface := range commands {
		// Parse command
		cmd, err := ParseCommand(cmdInterface)
		if err != nil {
			return results, fmt.Errorf("failed to parse post_deploy command %d: %w", i, err)
		}

		// Run command
		result, err := e.RunCommand(ctx, cmd, timeout)
		results = append(results, result)

		if err != nil {
			return results, fmt.Errorf("post_deploy command %d failed: %w (command: %s)",
				i, err, strings.Join(cmd[:min(3, len(cmd))], " "))
		}

		if !result.OK() {
			return results, fmt.Errorf("post_deploy command %d exited with code %d (command: %s)",
				i, result.ReturnCode, strings.Join(cmd[:min(3, len(cmd))], " "))
		}
	}

	return results, nil
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
