package cmdutil

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/kballard/go-shellquote"
)

// ExecOptions configures command execution.
type ExecOptions struct {
	// Dir is the working directory for the command.
	Dir string

	// Timeout is the maximum execution time.
	// If zero, no timeout is applied.
	Timeout time.Duration

	// Env contains environment variables for the command.
	// Each entry should be in the form "KEY=value".
	Env []string

	// CombinedOutput determines if stdout and stderr are combined.
	// Default: true
	CombinedOutput bool
}

// Result contains the result of a command execution.
type Result struct {
	// Stdout is the standard output (only if CombinedOutput is false).
	Stdout []byte

	// Stderr is the standard error (only if CombinedOutput is false).
	Stderr []byte

	// Output is the combined stdout and stderr (only if CombinedOutput is true).
	Output []byte

	// ExitCode is the exit code of the command.
	ExitCode int

	// Duration is how long the command took to execute.
	Duration time.Duration
}

// Run executes a command with the given options.
// The command is provided as a slice of arguments (command and its arguments).
// Returns the result or an error if the command fails.
func Run(ctx context.Context, opts ExecOptions, cmdParts []string) (*Result, error) {
	if len(cmdParts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	// Apply timeout if specified
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	// Create command
	cmd := exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)
	cmd.Dir = opts.Dir
	cmd.Env = opts.Env

	// Track execution time
	start := time.Now()

	// Execute command
	var result Result
	var err error

	if opts.CombinedOutput {
		result.Output, err = cmd.CombinedOutput()
	} else {
		result.Stdout, err = cmd.Output()
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.Stderr = exitErr.Stderr
		}
	}

	result.Duration = time.Since(start)

	// Get exit code
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	if err != nil {
		return &result, fmt.Errorf("command failed: %w", err)
	}

	return &result, nil
}

// RunSimple executes a command with default options (combined output, no timeout).
// This is a convenience wrapper around Run for simple use cases.
func RunSimple(ctx context.Context, workDir string, cmdParts []string) ([]byte, error) {
	result, err := Run(ctx, ExecOptions{
		Dir:            workDir,
		CombinedOutput: true,
	}, cmdParts)

	if err != nil {
		return result.Output, err
	}

	return result.Output, nil
}

// RunWithTimeout executes a command with a timeout.
// This is a convenience wrapper around Run.
func RunWithTimeout(ctx context.Context, workDir string, timeout time.Duration, cmdParts []string) ([]byte, error) {
	result, err := Run(ctx, ExecOptions{
		Dir:            workDir,
		Timeout:        timeout,
		CombinedOutput: true,
	}, cmdParts)

	if err != nil {
		return result.Output, err
	}

	return result.Output, nil
}

// ParseCommandString parses a shell-quoted command string into parts.
// This is useful when commands are stored as strings with proper quoting.
//
// Example:
//   "git commit -m \"my message\"" -> ["git", "commit", "-m", "my message"]
func ParseCommandString(cmdStr string) ([]string, error) {
	parts, err := shellquote.Split(cmdStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse command string: %w", err)
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty command string")
	}
	return parts, nil
}

// ParseCommandList parses a command that can be either a string or a list.
// This handles the two formats from YAML configuration:
//   - String format: "npm install --production"
//   - List format: ["npm", "install", "--production"]
func ParseCommandList(cmd interface{}) ([]string, error) {
	switch v := cmd.(type) {
	case string:
		return ParseCommandString(v)
	case []interface{}:
		// Convert []interface{} to []string
		parts := make([]string, len(v))
		for i, item := range v {
			str, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("command list item %d is not a string: %T", i, item)
			}
			parts[i] = str
		}
		if len(parts) == 0 {
			return nil, fmt.Errorf("empty command list")
		}
		return parts, nil
	case []string:
		if len(v) == 0 {
			return nil, fmt.Errorf("empty command list")
		}
		return v, nil
	default:
		return nil, fmt.Errorf("invalid command type: %T (must be string or list)", cmd)
	}
}

// FormatCommand formats command parts into a readable string for logging.
// Example: ["git", "commit", "-m", "my message"] -> "git commit -m 'my message'"
func FormatCommand(cmdParts []string) string {
	if len(cmdParts) == 0 {
		return "<empty command>"
	}

	// Quote arguments that contain spaces or special characters
	quoted := make([]string, len(cmdParts))
	for i, part := range cmdParts {
		if strings.ContainsAny(part, " \t\n\"'") {
			quoted[i] = shellquote.Join(part)
		} else {
			quoted[i] = part
		}
	}

	return strings.Join(quoted, " ")
}

// SanitizeOutput removes sensitive information from command output.
// This is useful for logging command output without exposing secrets.
func SanitizeOutput(output []byte, secrets []string) []byte {
	sanitized := string(output)
	for _, secret := range secrets {
		if secret != "" && len(secret) > 0 {
			sanitized = strings.ReplaceAll(sanitized, secret, "***REDACTED***")
		}
	}
	return []byte(sanitized)
}
