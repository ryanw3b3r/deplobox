package security

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// DefaultAllowedCommands is the default set of commands allowed for deployment operations.
var DefaultAllowedCommands = map[string]bool{
	"git":      true,
	"composer": true,
	"npm":      true,
	"npx":      true,
	"yarn":     true,
	"pnpm":     true,
	"php":      true,
	"pm2":      true,
	"node":     true,
	"python":   true,
	"python3":  true,
	"pip":      true,
	"pip3":     true,
	"bundle":   true,
	"rake":     true,
	"rails":    true,
	"artisan":  true, // Laravel artisan (via php artisan)
	"make":     true,
	"cargo":    true,
	"go":       true,
	"docker":   true,
	"rsync":    true,
	"cp":       true,
	"mv":       true,
	"ln":       true,
	"chmod":    true,
	"chown":    true,
}

// SandboxedExecutor provides safe command execution with validation and sandboxing.
type SandboxedExecutor struct {
	// AllowedCommands is the map of commands that are permitted to run.
	AllowedCommands map[string]bool

	// WorkDir is the working directory for command execution.
	WorkDir string

	// Env contains environment variables for the command.
	Env []string

	// AllowShellMetachars allows shell metacharacters in arguments (DANGEROUS!).
	// This should almost always be false.
	AllowShellMetachars bool
}

// NewSandboxedExecutor creates a new sandboxed executor with default settings.
func NewSandboxedExecutor(workDir string) *SandboxedExecutor {
	return &SandboxedExecutor{
		AllowedCommands:     DefaultAllowedCommands,
		WorkDir:             workDir,
		AllowShellMetachars: false,
	}
}

// Execute runs a command with validation and sandboxing.
// Returns the combined stdout/stderr output and any error.
func (e *SandboxedExecutor) Execute(ctx context.Context, cmdParts []string) ([]byte, error) {
	if len(cmdParts) == 0 {
		return nil, fmt.Errorf("empty command")
	}

	baseCmd := cmdParts[0]

	// Validate command is allowed
	if !e.AllowedCommands[baseCmd] {
		return nil, fmt.Errorf("command not allowed: %s (must be one of: %v)",
			baseCmd, e.getAllowedCommandsList())
	}

	// Prevent shell metacharacters in arguments unless explicitly allowed
	if !e.AllowShellMetachars {
		for i, arg := range cmdParts[1:] {
			if containsShellMetachars(arg) {
				return nil, fmt.Errorf("argument %d contains shell metacharacters: %s", i+1, arg)
			}
		}
	}

	// Create command without shell (prevents shell injection)
	cmd := exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)
	cmd.Dir = e.WorkDir
	cmd.Env = e.Env

	// Run command and capture output
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("command failed: %w", err)
	}

	return output, nil
}

// ExecuteQuiet runs a command and discards the output (but still checks for errors).
func (e *SandboxedExecutor) ExecuteQuiet(ctx context.Context, cmdParts []string) error {
	_, err := e.Execute(ctx, cmdParts)
	return err
}

// getAllowedCommandsList returns a sorted list of allowed commands for error messages.
func (e *SandboxedExecutor) getAllowedCommandsList() []string {
	commands := make([]string, 0, len(e.AllowedCommands))
	for cmd := range e.AllowedCommands {
		commands = append(commands, cmd)
	}
	return commands
}

// containsShellMetachars checks if a string contains shell metacharacters.
// These characters can be used for command injection attacks.
func containsShellMetachars(s string) bool {
	dangerous := []string{
		";",  // Command separator
		"|",  // Pipe
		"&",  // Background/AND
		"$",  // Variable expansion
		"`",  // Command substitution
		"\n", // Newline (command separator)
		">",  // Redirect output
		"<",  // Redirect input
		"(",  // Subshell start
		")",  // Subshell end
		"{",  // Brace expansion start
		"}",  // Brace expansion end
		"*",  // Glob wildcard
		"?",  // Glob single char
		"[",  // Glob character class
		"]",  // Glob character class end
		"\\", // Escape character
		"'",  // Single quote (can bypass some protections)
		"\"", // Double quote (can bypass some protections)
	}

	for _, char := range dangerous {
		if strings.Contains(s, char) {
			return true
		}
	}

	return false
}

// ValidateCommandParts validates a command before execution.
// This can be used to pre-validate commands without executing them.
func (e *SandboxedExecutor) ValidateCommandParts(cmdParts []string) error {
	if len(cmdParts) == 0 {
		return fmt.Errorf("empty command")
	}

	baseCmd := cmdParts[0]

	// Validate command is allowed
	if !e.AllowedCommands[baseCmd] {
		return fmt.Errorf("command not allowed: %s", baseCmd)
	}

	// Check for shell metacharacters
	if !e.AllowShellMetachars {
		for i, arg := range cmdParts[1:] {
			if containsShellMetachars(arg) {
				return fmt.Errorf("argument %d contains shell metacharacters: %s", i+1, arg)
			}
		}
	}

	return nil
}

// AddAllowedCommand adds a command to the allowed list.
// Use with caution - only add commands you trust.
func (e *SandboxedExecutor) AddAllowedCommand(cmd string) {
	if e.AllowedCommands == nil {
		e.AllowedCommands = make(map[string]bool)
	}
	e.AllowedCommands[cmd] = true
}

// RemoveAllowedCommand removes a command from the allowed list.
func (e *SandboxedExecutor) RemoveAllowedCommand(cmd string) {
	delete(e.AllowedCommands, cmd)
}

// IsCommandAllowed checks if a command is in the allowed list.
func (e *SandboxedExecutor) IsCommandAllowed(cmd string) bool {
	return e.AllowedCommands[cmd]
}
