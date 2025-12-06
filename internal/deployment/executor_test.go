package deployment

import (
	"context"
	"testing"
)

func TestParseCommand_String(t *testing.T) {
	testCases := []struct {
		input    string
		expected []string
	}{
		{"echo hello", []string{"echo", "hello"}},
		{"git pull origin main", []string{"git", "pull", "origin", "main"}},
		{`echo "hello world"`, []string{"echo", "hello world"}},
		{"cmd arg1 arg2 arg3", []string{"cmd", "arg1", "arg2", "arg3"}},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result, err := ParseCommand(tc.input)
			if err != nil {
				t.Fatalf("ParseCommand(%q) error: %v", tc.input, err)
			}

			if len(result) != len(tc.expected) {
				t.Errorf("ParseCommand(%q) = %v, expected %v", tc.input, result, tc.expected)
				return
			}

			for i := range result {
				if result[i] != tc.expected[i] {
					t.Errorf("ParseCommand(%q)[%d] = %q, expected %q", tc.input, i, result[i], tc.expected[i])
				}
			}
		})
	}
}

func TestParseCommand_List(t *testing.T) {
	testCases := []struct {
		name     string
		input    []interface{}
		expected []string
	}{
		{
			name:     "simple list",
			input:    []interface{}{"echo", "hello"},
			expected: []string{"echo", "hello"},
		},
		{
			name:     "git command",
			input:    []interface{}{"git", "pull", "origin", "main"},
			expected: []string{"git", "pull", "origin", "main"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseCommand(tc.input)
			if err != nil {
				t.Fatalf("ParseCommand(%v) error: %v", tc.input, err)
			}

			if len(result) != len(tc.expected) {
				t.Errorf("ParseCommand(%v) = %v, expected %v", tc.input, result, tc.expected)
				return
			}

			for i := range result {
				if result[i] != tc.expected[i] {
					t.Errorf("ParseCommand(%v)[%d] = %q, expected %q", tc.input, i, result[i], tc.expected[i])
				}
			}
		})
	}
}

func TestParseCommand_InvalidType(t *testing.T) {
	testCases := []struct {
		name  string
		input interface{}
	}{
		{"integer", 123},
		{"float", 3.14},
		{"bool", true},
		{"nil", nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseCommand(tc.input)
			if err == nil {
				t.Errorf("Expected ParseCommand(%v) to return error", tc.input)
			}
		})
	}
}

func TestExecutor_RunCommand_Success(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewExecutor(tmpDir)

	ctx := context.Background()
	result, err := executor.RunCommand(ctx, []string{"echo", "test"}, 5)

	if err != nil {
		t.Fatalf("RunCommand error: %v", err)
	}

	if !result.OK() {
		t.Errorf("Expected command to succeed, got return code %d", result.ReturnCode)
	}

	if result.Stdout != "test\n" {
		t.Errorf("Expected stdout 'test\\n', got %q", result.Stdout)
	}
}

func TestExecutor_RunCommand_Failure(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewExecutor(tmpDir)

	ctx := context.Background()
	result, err := executor.RunCommand(ctx, []string{"false"}, 5)

	// Command should complete without error (just non-zero exit)
	if err != nil {
		t.Fatalf("RunCommand error: %v", err)
	}

	if result.OK() {
		t.Error("Expected command to fail (non-zero exit code)")
	}

	if result.ReturnCode != 1 {
		t.Errorf("Expected return code 1, got %d", result.ReturnCode)
	}
}

func TestExecutor_RunCommand_Timeout(t *testing.T) {
	tmpDir := t.TempDir()
	executor := NewExecutor(tmpDir)

	ctx := context.Background()
	// Sleep for 10 seconds with 1 second timeout
	result, err := executor.RunCommand(ctx, []string{"sleep", "10"}, 1)

	// Either error or non-zero return code indicates timeout
	if err == nil && result.OK() {
		t.Error("Expected timeout error or non-zero exit code")
	}
}

func TestExecutionResult_OK(t *testing.T) {
	testCases := []struct {
		returnCode int
		expected   bool
	}{
		{0, true},
		{1, false},
		{127, false},
		{-1, false},
	}

	for _, tc := range testCases {
		result := &ExecutionResult{ReturnCode: tc.returnCode}
		if result.OK() != tc.expected {
			t.Errorf("ExecutionResult{ReturnCode: %d}.OK() = %v, expected %v",
				tc.returnCode, result.OK(), tc.expected)
		}
	}
}
