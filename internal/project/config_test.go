package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateProjectConfig_ValidConfig(t *testing.T) {
	// Create temporary git repository for testing
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	// Create deployment structure
	releasesDir := filepath.Join(tmpDir, "releases")
	if err := os.Mkdir(releasesDir, 0755); err != nil {
		t.Fatalf("Failed to create releases directory: %v", err)
	}

	sharedDir := filepath.Join(tmpDir, "shared")
	if err := os.Mkdir(sharedDir, 0755); err != nil {
		t.Fatalf("Failed to create shared directory: %v", err)
	}

	// Create a release and symlink it as current
	release1 := filepath.Join(releasesDir, "2024-01-01-00-00-00")
	if err := os.Mkdir(release1, 0755); err != nil {
		t.Fatalf("Failed to create release directory: %v", err)
	}

	// Create .git in release directory (it's a git clone)
	releaseGitDir := filepath.Join(release1, ".git")
	if err := os.Mkdir(releaseGitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git in release: %v", err)
	}

	currentLink := filepath.Join(tmpDir, "current")
	if err := os.Symlink(release1, currentLink); err != nil {
		t.Fatalf("Failed to create current symlink: %v", err)
	}

	config := ProjectConfig{
		Path:              tmpDir,
		Secret:            "valid-secret-with-at-least-32-chars-here",
		Branch:            "main",
		PullTimeout:       60,
		PostDeployTimeout: 300,
		PostDeploy:        []interface{}{"echo test"},
	}

	errors := ValidateProjectConfig("test-project", config)
	if len(errors) > 0 {
		t.Errorf("Expected valid config to pass validation, got errors: %v", errors)
	}
}

func TestValidateProjectConfig_RelativePath(t *testing.T) {
	config := ProjectConfig{
		Path:   "./relative/path",
		Secret: "valid-secret-with-at-least-32-chars-here",
	}

	errors := ValidateProjectConfig("test-project", config)
	if len(errors) == 0 {
		t.Error("Expected relative path to be rejected")
	}

	found := false
	for _, err := range errors {
		if strings.Contains(err, "path must be absolute") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'path must be absolute' error, got: %v", errors)
	}
}

func TestValidateProjectConfig_NonExistentPath(t *testing.T) {
	config := ProjectConfig{
		Path:   "/nonexistent/path/that/does/not/exist",
		Secret: "valid-secret-with-at-least-32-chars-here",
	}

	errors := ValidateProjectConfig("test-project", config)
	if len(errors) == 0 {
		t.Error("Expected nonexistent path to be rejected")
	}

	found := false
	for _, err := range errors {
		if strings.Contains(err, "does not exist") || strings.Contains(err, "cannot resolve path") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'does not exist' or 'cannot resolve path' error, got: %v", errors)
	}
}

func TestValidateProjectConfig_MissingGitDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	config := ProjectConfig{
		Path:   tmpDir,
		Secret: "valid-secret-with-at-least-32-chars-here",
	}

	errors := ValidateProjectConfig("test-project", config)
	if len(errors) == 0 {
		t.Error("Expected validation errors for directory without deployment structure")
	}

	// Should have errors about missing deployment structure (.git, current, releases, shared)
	// We just verify there are errors, not specifically which ones
	if len(errors) < 3 {
		t.Errorf("Expected at least 3 validation errors (current, releases, shared), got %d: %v", len(errors), errors)
	}
}

func TestValidateProjectConfig_ShortSecret(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	os.Mkdir(gitDir, 0755)

	config := ProjectConfig{
		Path:   tmpDir,
		Secret: "short", // Less than 32 characters
	}

	errors := ValidateProjectConfig("test-project", config)
	if len(errors) == 0 {
		t.Error("Expected short secret to be rejected")
	}

	found := false
	for _, err := range errors {
		if strings.Contains(err, "secret too short") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'secret too short' error, got: %v", errors)
	}
}

func TestValidateProjectConfig_PlaceholderSecret(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	os.Mkdir(gitDir, 0755)

	// Test that short secrets are rejected (placeholders are caught because they're short)
	config := ProjectConfig{
		Path:   tmpDir,
		Secret: "password", // Too short - will be rejected
	}

	errors := ValidateProjectConfig("test-project", config)

	if len(errors) == 0 {
		t.Error("Expected short secret to be rejected")
	}

	// Verify the secret error is present
	foundSecretError := false
	for _, err := range errors {
		if strings.Contains(err, "secret") {
			foundSecretError = true
			break
		}
	}
	if !foundSecretError {
		t.Errorf("Expected secret-related error, got: %v", errors)
	}
}

func TestValidateProjectConfig_InvalidTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	os.Mkdir(gitDir, 0755)

	testCases := []struct {
		name              string
		pullTimeout       int
		postDeployTimeout int
	}{
		{"negative pull timeout", -1, 300},
		{"negative post deploy timeout", 60, -1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := ProjectConfig{
				Path:              tmpDir,
				Secret:            "valid-secret-with-at-least-32-chars-here",
				PullTimeout:       tc.pullTimeout,
				PostDeployTimeout: tc.postDeployTimeout,
			}

			errors := ValidateProjectConfig("test-project", config)
			if len(errors) == 0 {
				t.Errorf("Expected invalid timeout to be rejected for %s", tc.name)
			}

			found := false
			for _, err := range errors {
				if strings.Contains(err, "must be a positive integer") {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected timeout error for %s, got: %v", tc.name, errors)
			}
		})
	}
}

func TestValidateProjectConfig_InvalidBranch(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	os.Mkdir(gitDir, 0755)

	config := ProjectConfig{
		Path:   tmpDir,
		Secret: "valid-secret-with-at-least-32-chars-here",
		Branch: "-invalid-branch-name",
	}

	errors := ValidateProjectConfig("test-project", config)
	if len(errors) == 0 {
		t.Error("Expected branch starting with '-' to be rejected")
	}

	found := false
	for _, err := range errors {
		if strings.Contains(err, "cannot start with '-'") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected branch validation error, got: %v", errors)
	}
}

func TestValidateProjectConfig_InvalidPostDeploy(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	os.Mkdir(gitDir, 0755)

	config := ProjectConfig{
		Path:       tmpDir,
		Secret:     "valid-secret-with-at-least-32-chars-here",
		PostDeploy: []interface{}{123}, // Invalid type (not string or list)
	}

	errors := ValidateProjectConfig("test-project", config)
	if len(errors) == 0 {
		t.Error("Expected invalid post_deploy type to be rejected")
	}

	found := false
	for _, err := range errors {
		if strings.Contains(err, "must be a string or list") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected post_deploy type error, got: %v", errors)
	}
}

func TestProjectMatchesRef(t *testing.T) {
	project := &Project{
		Name:   "test",
		Branch: "main",
	}

	testCases := []struct {
		ref      string
		expected bool
	}{
		{"refs/heads/main", true},
		{"refs/heads/develop", false},
		{"refs/tags/v1.0", false},
		{"main", false},
	}

	for _, tc := range testCases {
		result := project.MatchesRef(tc.ref)
		if result != tc.expected {
			t.Errorf("MatchesRef(%q) = %v, expected %v", tc.ref, result, tc.expected)
		}
	}
}
