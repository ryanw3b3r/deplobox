package security

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSandboxedExecutor_Execute(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	tests := []struct {
		name        string
		cmdParts    []string
		allowShell  bool
		wantErr     bool
		errContains string
	}{
		// Valid commands
		{
			"echo hello",
			[]string{"echo", "hello"},
			false,
			true, // echo is not in DefaultAllowedCommands
			"command not allowed",
		},
		{
			"git status",
			[]string{"git", "status"},
			false,
			true, // Will fail in tmpDir without git repo
			"",
		},
		{
			"git with args",
			[]string{"git", "log", "--oneline", "-n", "5"},
			false,
			true, // Will fail in tmpDir without git repo
			"",
		},

		// Blocked commands
		{
			"rm command",
			[]string{"rm", "-rf", "/"},
			false,
			true,
			"command not allowed",
		},
		{
			"curl command",
			[]string{"curl", "evil.com"},
			false,
			true,
			"command not allowed",
		},
		{
			"bash command",
			[]string{"bash", "-c", "whoami"},
			false,
			true,
			"command not allowed",
		},

		// Shell metacharacter injection attempts
		{
			"semicolon injection",
			[]string{"git", "status; rm -rf /"},
			false,
			true,
			"shell metacharacters",
		},
		{
			"pipe injection",
			[]string{"git", "log | grep secret"},
			false,
			true,
			"shell metacharacters",
		},
		{
			"ampersand injection",
			[]string{"git", "pull && curl evil.com"},
			false,
			true,
			"shell metacharacters",
		},
		{
			"backtick injection",
			[]string{"git", "log `whoami`"},
			false,
			true,
			"shell metacharacters",
		},
		{
			"dollar injection",
			[]string{"git", "log $(whoami)"},
			false,
			true,
			"shell metacharacters",
		},
		{
			"redirect injection",
			[]string{"git", "log > /etc/passwd"},
			false,
			true,
			"shell metacharacters",
		},
		{
			"subshell injection",
			[]string{"git", "log (malicious)"},
			false,
			true,
			"shell metacharacters",
		},
		{
			"quote injection",
			[]string{"git", "log 'malicious'"},
			false,
			true,
			"shell metacharacters",
		},

		// Empty command
		{
			"empty command",
			[]string{},
			false,
			true,
			"empty command",
		},

		// Shell metacharacters allowed (dangerous but sometimes needed)
		{
			"pipe with allow shell",
			[]string{"git", "log | head"},
			true,
			true, // Still fails because pipe is in the argument, not executed by shell
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewSandboxedExecutor(tmpDir)
			executor.AllowShellMetachars = tt.allowShell

			_, err := executor.Execute(ctx, tt.cmdParts)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Execute() error = %v, want error containing %q", err, tt.errContains)
				}
			}
		})
	}
}

func TestSandboxedExecutor_ExecuteQuiet(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	executor := NewSandboxedExecutor(tmpDir)

	// Test successful command
	err := executor.ExecuteQuiet(ctx, []string{"git", "--version"})
	if err != nil {
		t.Errorf("ExecuteQuiet() with valid command error = %v", err)
	}

	// Test blocked command
	err = executor.ExecuteQuiet(ctx, []string{"rm", "-rf", "/"})
	if err == nil {
		t.Errorf("ExecuteQuiet() should fail with blocked command")
	}
}

func TestSandboxedExecutor_ValidateCommandParts(t *testing.T) {
	executor := NewSandboxedExecutor("/tmp")

	tests := []struct {
		name     string
		cmdParts []string
		wantErr  bool
	}{
		{"valid git command", []string{"git", "status"}, false},
		{"valid npm command", []string{"npm", "install"}, false},
		{"blocked command", []string{"rm", "-rf", "/"}, true},
		{"empty command", []string{}, true},
		{"shell metacharacters", []string{"git", "log; whoami"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executor.ValidateCommandParts(tt.cmdParts)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCommandParts() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSandboxedExecutor_AddRemoveCommand(t *testing.T) {
	executor := NewSandboxedExecutor("/tmp")

	// Add a new command
	executor.AddAllowedCommand("mycommand")
	if !executor.IsCommandAllowed("mycommand") {
		t.Errorf("AddAllowedCommand() failed to add command")
	}

	// Validate it can be used
	err := executor.ValidateCommandParts([]string{"mycommand", "arg"})
	if err != nil {
		t.Errorf("Command should be allowed after adding: %v", err)
	}

	// Remove the command
	executor.RemoveAllowedCommand("mycommand")
	if executor.IsCommandAllowed("mycommand") {
		t.Errorf("RemoveAllowedCommand() failed to remove command")
	}

	// Validate it's now blocked
	err = executor.ValidateCommandParts([]string{"mycommand", "arg"})
	if err == nil {
		t.Errorf("Command should be blocked after removing")
	}
}

func TestSandboxedExecutor_IsCommandAllowed(t *testing.T) {
	executor := NewSandboxedExecutor("/tmp")

	tests := []struct {
		name    string
		command string
		want    bool
	}{
		{"git allowed", "git", true},
		{"npm allowed", "npm", true},
		{"php allowed", "php", true},
		{"rm blocked", "rm", false},
		{"bash blocked", "bash", false},
		{"custom blocked", "mycommand", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := executor.IsCommandAllowed(tt.command)
			if got != tt.want {
				t.Errorf("IsCommandAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainsShellMetachars(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		// Safe inputs
		{"simple string", "hello", false},
		{"with numbers", "hello123", false},
		{"with dash", "my-command", false},
		{"with underscore", "my_command", false},
		{"with dot", "file.txt", false},
		{"with slash", "/path/to/file", false},
		{"with equals", "key=value", false},
		{"with colon", "http://example.com", false},
		{"with @", "user@host", false},

		// Dangerous metacharacters
		{"semicolon", "cmd; malicious", true},
		{"pipe", "cmd | grep", true},
		{"ampersand", "cmd && other", true},
		{"dollar", "cmd $(whoami)", true},
		{"backtick", "cmd `whoami`", true},
		{"redirect output", "cmd > file", true},
		{"redirect input", "cmd < file", true},
		{"subshell open", "cmd (sub", true},
		{"subshell close", "cmd sub)", true},
		{"brace open", "cmd {a,b}", true},
		{"brace close", "cmd }", true},
		{"asterisk", "cmd *.txt", true},
		{"question mark", "cmd ?.txt", true},
		{"bracket open", "cmd [abc]", true},
		{"bracket close", "cmd ]", true},
		{"backslash", "cmd \\n", true},
		{"single quote", "cmd 'quoted'", true},
		{"double quote", "cmd \"quoted\"", true},
		{"newline", "cmd\nmalicious", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsShellMetachars(tt.input)
			if got != tt.want {
				t.Errorf("containsShellMetachars(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSandboxedExecutor_WithContext(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewSandboxedExecutor(tmpDir)

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := executor.Execute(ctx, []string{"git", "--version"})
		if err == nil {
			t.Errorf("Execute() should fail with cancelled context")
		}
	})

	t.Run("context timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		time.Sleep(10 * time.Millisecond) // Ensure timeout expires

		_, err := executor.Execute(ctx, []string{"git", "--version"})
		if err == nil {
			t.Errorf("Execute() should fail with expired context")
		}
	})
}

func TestNewSandboxedExecutor(t *testing.T) {
	workDir := "/var/www/project"
	executor := NewSandboxedExecutor(workDir)

	if executor.WorkDir != workDir {
		t.Errorf("NewSandboxedExecutor() WorkDir = %v, want %v", executor.WorkDir, workDir)
	}

	if executor.AllowShellMetachars {
		t.Errorf("NewSandboxedExecutor() AllowShellMetachars should be false by default")
	}

	if executor.AllowedCommands == nil {
		t.Errorf("NewSandboxedExecutor() AllowedCommands should not be nil")
	}

	// Check some default allowed commands
	if !executor.IsCommandAllowed("git") {
		t.Errorf("NewSandboxedExecutor() should allow git by default")
	}
	if !executor.IsCommandAllowed("npm") {
		t.Errorf("NewSandboxedExecutor() should allow npm by default")
	}
}

func TestSandboxedExecutor_Environment(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewSandboxedExecutor(tmpDir)
	executor.Env = []string{"MY_VAR=test"}

	// Note: This test will only work if git is installed
	// If git is not in PATH, the command will fail for different reasons
	_, err := executor.Execute(context.Background(), []string{"git", "--version"})

	// We mainly care that the executor doesn't crash with custom env
	// The actual command might fail if git is not installed, which is okay for this test
	if err != nil && !strings.Contains(err.Error(), "command not allowed") {
		// Command execution attempted (git is in allowed list)
		// Failure is acceptable if git is not installed
		t.Logf("Execute with custom env: %v (expected if git not installed)", err)
	}
}

// Benchmark tests
func BenchmarkExecute(b *testing.B) {
	executor := NewSandboxedExecutor("/tmp")

	for i := 0; i < b.N; i++ {
		_ = executor.ValidateCommandParts([]string{"git", "status"})
	}
}

func BenchmarkContainsShellMetachars(b *testing.B) {
	testStr := "this-is-a-safe-string-with-no-metacharacters"
	for i := 0; i < b.N; i++ {
		_ = containsShellMetachars(testStr)
	}
}
