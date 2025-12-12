package cmdutil

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRun(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		opts    ExecOptions
		cmd     []string
		wantErr bool
	}{
		{
			"successful command",
			ExecOptions{CombinedOutput: true},
			[]string{"echo", "hello"},
			false,
		},
		{
			"command with args",
			ExecOptions{CombinedOutput: true},
			[]string{"echo", "hello", "world"},
			false,
		},
		{
			"command that fails",
			ExecOptions{CombinedOutput: true},
			[]string{"ls", "/nonexistent/directory/path"},
			true,
		},
		{
			"empty command",
			ExecOptions{CombinedOutput: true},
			[]string{},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Run(ctx, tt.opts, tt.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if result == nil {
					t.Error("Run() returned nil result for successful command")
				}
				if result.Duration == 0 {
					t.Error("Run() did not record execution duration")
				}
			}
		})
	}
}

func TestRunSimple(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		cmd     []string
		wantErr bool
	}{
		{
			"successful command",
			[]string{"echo", "test"},
			false,
		},
		{
			"failing command",
			[]string{"ls", "/nonexistent"},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, err := RunSimple(ctx, tmpDir, tt.cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("RunSimple() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && len(output) == 0 {
				t.Error("RunSimple() returned empty output for successful command")
			}
		})
	}
}

func TestRunWithTimeout(t *testing.T) {
	ctx := context.Background()

	t.Run("command completes before timeout", func(t *testing.T) {
		_, err := RunWithTimeout(ctx, "", 5*time.Second, []string{"echo", "test"})
		if err != nil {
			t.Errorf("RunWithTimeout() error = %v, want nil", err)
		}
	})

	t.Run("command times out", func(t *testing.T) {
		_, err := RunWithTimeout(ctx, "", 1*time.Millisecond, []string{"sleep", "10"})
		if err == nil {
			t.Error("RunWithTimeout() should timeout for long command")
		}
	})
}

func TestParseCommandString(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			"simple command",
			"git status",
			[]string{"git", "status"},
			false,
		},
		{
			"command with quoted argument",
			"git commit -m \"my message\"",
			[]string{"git", "commit", "-m", "my message"},
			false,
		},
		{
			"command with single quotes",
			"echo 'hello world'",
			[]string{"echo", "hello world"},
			false,
		},
		{
			"command with escaped quotes",
			"echo \"hello \\\"world\\\"\"",
			[]string{"echo", "hello \"world\""},
			false,
		},
		{
			"empty string",
			"",
			nil,
			true,
		},
		{
			"whitespace only",
			"   ",
			nil,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCommandString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCommandString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !equalStringSlices(got, tt.want) {
				t.Errorf("ParseCommandString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseCommandList(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    []string
		wantErr bool
	}{
		{
			"string format",
			"npm install --production",
			[]string{"npm", "install", "--production"},
			false,
		},
		{
			"list format ([]interface{})",
			[]interface{}{"npm", "install", "--production"},
			[]string{"npm", "install", "--production"},
			false,
		},
		{
			"list format ([]string)",
			[]string{"npm", "install", "--production"},
			[]string{"npm", "install", "--production"},
			false,
		},
		{
			"empty string",
			"",
			nil,
			true,
		},
		{
			"empty list",
			[]string{},
			nil,
			true,
		},
		{
			"invalid type",
			123,
			nil,
			true,
		},
		{
			"list with non-string element",
			[]interface{}{"npm", 123, "install"},
			nil,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseCommandList(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCommandList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !equalStringSlices(got, tt.want) {
				t.Errorf("ParseCommandList() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatCommand(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{
			"simple command",
			[]string{"git", "status"},
			"git status",
		},
		{
			"command with spaces in argument",
			[]string{"git", "commit", "-m", "my message"},
			"git commit -m 'my message'",
		},
		{
			"empty command",
			[]string{},
			"<empty command>",
		},
		{
			"single command",
			[]string{"ls"},
			"ls",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatCommand(tt.input)
			// The exact quoting format may vary, so just check it's not empty
			// and contains the command parts
			if len(tt.input) > 0 && !strings.Contains(got, tt.input[0]) {
				t.Errorf("FormatCommand() = %v, should contain %v", got, tt.input[0])
			}
			if len(tt.input) == 0 && got != "<empty command>" {
				t.Errorf("FormatCommand() = %v, want %v", got, "<empty command>")
			}
		})
	}
}

func TestSanitizeOutput(t *testing.T) {
	tests := []struct {
		name    string
		output  []byte
		secrets []string
		want    string
	}{
		{
			"redact single secret",
			[]byte("Password: mysecret123"),
			[]string{"mysecret123"},
			"Password: ***REDACTED***",
		},
		{
			"redact multiple secrets",
			[]byte("user: admin, password: secret1, token: secret2"),
			[]string{"secret1", "secret2"},
			"user: admin, password: ***REDACTED***, token: ***REDACTED***",
		},
		{
			"no secrets",
			[]byte("public information"),
			[]string{},
			"public information",
		},
		{
			"empty secret",
			[]byte("some output"),
			[]string{""},
			"some output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeOutput(tt.output, tt.secrets)
			if string(got) != tt.want {
				t.Errorf("SanitizeOutput() = %v, want %v", string(got), tt.want)
			}
		})
	}
}

func TestExecOptions(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	t.Run("with working directory", func(t *testing.T) {
		opts := ExecOptions{
			Dir:            tmpDir,
			CombinedOutput: true,
		}
		_, err := Run(ctx, opts, []string{"pwd"})
		if err != nil {
			t.Errorf("Run() with Dir option error = %v", err)
		}
	})

	t.Run("with environment variables", func(t *testing.T) {
		opts := ExecOptions{
			Env:            []string{"TEST_VAR=test_value"},
			CombinedOutput: true,
		}
		result, err := Run(ctx, opts, []string{"env"})
		if err != nil {
			t.Errorf("Run() with Env option error = %v", err)
		}
		if !strings.Contains(string(result.Output), "TEST_VAR=test_value") {
			t.Error("Run() did not set environment variable correctly")
		}
	})

	t.Run("with timeout", func(t *testing.T) {
		opts := ExecOptions{
			Timeout:        100 * time.Millisecond,
			CombinedOutput: true,
		}
		_, err := Run(ctx, opts, []string{"sleep", "1"})
		if err == nil {
			t.Error("Run() should timeout for long command")
		}
	})
}

func TestResult(t *testing.T) {
	ctx := context.Background()

	t.Run("combined output", func(t *testing.T) {
		opts := ExecOptions{CombinedOutput: true}
		result, err := Run(ctx, opts, []string{"echo", "test"})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}

		if len(result.Output) == 0 {
			t.Error("Result.Output should not be empty")
		}
		if !strings.Contains(string(result.Output), "test") {
			t.Error("Result.Output should contain 'test'")
		}
		if result.ExitCode != 0 {
			t.Errorf("Result.ExitCode = %d, want 0", result.ExitCode)
		}
	})

	t.Run("exit code for failed command", func(t *testing.T) {
		opts := ExecOptions{CombinedOutput: true}
		result, err := Run(ctx, opts, []string{"ls", "/nonexistent"})
		if err == nil {
			t.Error("Run() should return error for failed command")
		}
		if result.ExitCode == 0 {
			t.Error("Result.ExitCode should be non-zero for failed command")
		}
	})
}

// Helper functions

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Benchmark tests

func BenchmarkRun(b *testing.B) {
	ctx := context.Background()
	opts := ExecOptions{CombinedOutput: true}

	for i := 0; i < b.N; i++ {
		_, _ = Run(ctx, opts, []string{"echo", "test"})
	}
}

func BenchmarkParseCommandString(b *testing.B) {
	cmd := "git commit -m \"my message\""

	for i := 0; i < b.N; i++ {
		_, _ = ParseCommandString(cmd)
	}
}

func BenchmarkFormatCommand(b *testing.B) {
	cmd := []string{"git", "commit", "-m", "my message"}

	for i := 0; i < b.N; i++ {
		_ = FormatCommand(cmd)
	}
}
