package deployment

import (
	"context"
	"os"
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
	result, err := executor.RunCommand(ctx, []string{"echo", "test"}, 5, tmpDir)

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
	result, err := executor.RunCommand(ctx, []string{"false"}, 5, tmpDir)

	// Command should return error for non-zero exit
	if err == nil {
		t.Fatal("Expected RunCommand to return error for failed command")
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
	result, err := executor.RunCommand(ctx, []string{"sleep", "10"}, 1, tmpDir)

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

func TestExecutor_RestorePreviousRelease_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create releases directory
	releasesDir := tmpDir + "/releases"
	if err := os.MkdirAll(releasesDir, 0755); err != nil {
		t.Fatalf("Failed to create releases dir: %v", err)
	}

	// Create multiple releases with timestamp names (newest to oldest)
	releases := []string{
		"2024-12-09-15-30-00", // newest
		"2024-12-09-14-00-00",
		"2024-12-09-12-00-00", // oldest
	}

	for _, release := range releases {
		releaseDir := releasesDir + "/" + release
		if err := os.MkdirAll(releaseDir, 0755); err != nil {
			t.Fatalf("Failed to create release dir %s: %v", release, err)
		}
	}

	// Create current symlink pointing to the newest release
	currentLink := tmpDir + "/current"
	if err := os.Symlink("releases/"+releases[0], currentLink); err != nil {
		t.Fatalf("Failed to create current symlink: %v", err)
	}

	// Create executor and restore
	executor := NewExecutor(tmpDir)
	oldRelease, newRelease, err := executor.RestorePreviousRelease()

	if err != nil {
		t.Fatalf("RestorePreviousRelease error: %v", err)
	}

	// Verify old and new release names
	if oldRelease != releases[0] {
		t.Errorf("Expected old release %s, got %s", releases[0], oldRelease)
	}

	if newRelease != releases[1] {
		t.Errorf("Expected new release %s, got %s", releases[1], newRelease)
	}

	// Verify current symlink points to previous release
	linkTarget, err := os.Readlink(currentLink)
	if err != nil {
		t.Fatalf("Failed to read current symlink: %v", err)
	}

	expectedTarget := "releases/" + releases[1]
	if linkTarget != expectedTarget {
		t.Errorf("Expected symlink target %s, got %s", expectedTarget, linkTarget)
	}
}

func TestExecutor_RestorePreviousRelease_OnlyOneRelease(t *testing.T) {
	tmpDir := t.TempDir()

	// Create releases directory with only one release
	releasesDir := tmpDir + "/releases"
	releaseDir := releasesDir + "/2024-12-09-15-00-00"
	if err := os.MkdirAll(releaseDir, 0755); err != nil {
		t.Fatalf("Failed to create release dir: %v", err)
	}

	// Create current symlink
	currentLink := tmpDir + "/current"
	if err := os.Symlink("releases/2024-12-09-15-00-00", currentLink); err != nil {
		t.Fatalf("Failed to create current symlink: %v", err)
	}

	// Attempt to restore
	executor := NewExecutor(tmpDir)
	_, _, err := executor.RestorePreviousRelease()

	// Should fail because there's only one release
	if err == nil {
		t.Error("Expected error when only one release exists")
	}
}

func TestExecutor_RestorePreviousRelease_AlreadyOldest(t *testing.T) {
	tmpDir := t.TempDir()

	// Create releases directory
	releasesDir := tmpDir + "/releases"
	if err := os.MkdirAll(releasesDir, 0755); err != nil {
		t.Fatalf("Failed to create releases dir: %v", err)
	}

	// Create multiple releases
	releases := []string{
		"2024-12-09-15-00-00", // newest
		"2024-12-09-12-00-00", // oldest
	}

	for _, release := range releases {
		releaseDir := releasesDir + "/" + release
		if err := os.MkdirAll(releaseDir, 0755); err != nil {
			t.Fatalf("Failed to create release dir %s: %v", release, err)
		}
	}

	// Create current symlink pointing to the OLDEST release
	currentLink := tmpDir + "/current"
	if err := os.Symlink("releases/"+releases[1], currentLink); err != nil {
		t.Fatalf("Failed to create current symlink: %v", err)
	}

	// Attempt to restore
	executor := NewExecutor(tmpDir)
	_, _, err := executor.RestorePreviousRelease()

	// Should fail because current is already the oldest
	if err == nil {
		t.Error("Expected error when current release is already the oldest")
	}
}

func TestExecutor_RestorePreviousRelease_NoCurrentSymlink(t *testing.T) {
	tmpDir := t.TempDir()

	// Create releases directory but no current symlink
	releasesDir := tmpDir + "/releases"
	if err := os.MkdirAll(releasesDir, 0755); err != nil {
		t.Fatalf("Failed to create releases dir: %v", err)
	}

	// Attempt to restore
	executor := NewExecutor(tmpDir)
	_, _, err := executor.RestorePreviousRelease()

	// Should fail because current symlink doesn't exist
	if err == nil {
		t.Error("Expected error when current symlink doesn't exist")
	}
}

func TestExecutor_RunPostActivateCommands_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create current symlink directory structure
	currentDir := tmpDir + "/current"
	if err := os.MkdirAll(currentDir, 0755); err != nil {
		t.Fatalf("Failed to create current dir: %v", err)
	}

	executor := NewExecutor(tmpDir)
	ctx := context.Background()

	// Test with single command
	commands := []interface{}{"echo test"}
	results, err := executor.RunPostActivateCommands(ctx, commands, 5)

	if err != nil {
		t.Fatalf("RunPostActivateCommands error: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if !results[0].OK() {
		t.Errorf("Expected command to succeed, got return code %d", results[0].ReturnCode)
	}
}

func TestExecutor_RunPostActivateCommands_Multiple(t *testing.T) {
	tmpDir := t.TempDir()

	// Create current symlink directory structure
	currentDir := tmpDir + "/current"
	if err := os.MkdirAll(currentDir, 0755); err != nil {
		t.Fatalf("Failed to create current dir: %v", err)
	}

	executor := NewExecutor(tmpDir)
	ctx := context.Background()

	// Test with multiple commands
	commands := []interface{}{
		"echo first",
		"echo second",
		[]interface{}{"echo", "third"},
	}
	results, err := executor.RunPostActivateCommands(ctx, commands, 5)

	if err != nil {
		t.Fatalf("RunPostActivateCommands error: %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	for i, result := range results {
		if !result.OK() {
			t.Errorf("Expected command %d to succeed, got return code %d", i, result.ReturnCode)
		}
	}
}

func TestExecutor_RunPostActivateCommands_Failure(t *testing.T) {
	tmpDir := t.TempDir()

	// Create current symlink directory structure
	currentDir := tmpDir + "/current"
	if err := os.MkdirAll(currentDir, 0755); err != nil {
		t.Fatalf("Failed to create current dir: %v", err)
	}

	executor := NewExecutor(tmpDir)
	ctx := context.Background()

	// Test with failing command
	commands := []interface{}{"false"}
	_, err := executor.RunPostActivateCommands(ctx, commands, 5)

	if err == nil {
		t.Error("Expected error for failing command")
	}
}
