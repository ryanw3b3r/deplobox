package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateGitURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		// Valid cases
		{"valid github https", "https://github.com/user/repo", false},
		{"valid github https with .git", "https://github.com/user/repo.git", false},
		{"valid with dashes", "https://github.com/my-user/my-repo.git", false},
		{"valid with underscores", "https://github.com/my_user/my_repo.git", false},
		{"valid with numbers", "https://github.com/user123/repo456.git", false},
		{"valid with dots in repo", "https://github.com/user/repo.name.git", false},

		// Command injection attempts
		{"command injection semicolon", "https://github.com/user/repo.git; rm -rf /", true},
		{"command injection pipe", "https://github.com/user/repo.git | cat /etc/passwd", true},
		{"command injection ampersand", "https://github.com/user/repo.git && curl evil.com", true},
		{"command injection backtick", "https://github.com/user/repo`whoami`.git", true},
		{"command injection dollar", "https://github.com/user/repo$(whoami).git", true},

		// Path traversal attempts
		{"path traversal", "https://github.com/../../../etc/passwd", true},
		{"path traversal in repo", "https://github.com/user/../../../etc/passwd", true},

		// Invalid schemes
		{"http instead of https", "http://github.com/user/repo.git", true},
		{"git protocol", "git://github.com/user/repo.git", true},
		{"ssh protocol", "ssh://git@github.com/user/repo.git", true},
		{"no protocol", "github.com/user/repo.git", true},

		// Invalid hosts
		{"gitlab instead of github", "https://gitlab.com/user/repo.git", true},
		{"bitbucket", "https://bitbucket.org/user/repo.git", true},
		{"malicious host", "https://evil.github.com.attacker.com/user/repo.git", true},

		// Invalid formats
		{"empty url", "", true},
		{"missing repo", "https://github.com/user", true},
		{"missing user", "https://github.com/repo.git", true},
		{"special chars in user", "https://github.com/user@evil/repo.git", true},
		{"special chars in repo", "https://github.com/user/repo|evil.git", true},
		{"spaces in url", "https://github.com/user /repo.git", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGitURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateGitURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateBranchName(t *testing.T) {
	tests := []struct {
		name    string
		branch  string
		wantErr bool
	}{
		// Valid cases
		{"main branch", "main", false},
		{"master branch", "master", false},
		{"develop branch", "develop", false},
		{"feature branch", "feature/new-feature", false},
		{"release branch", "release/v1.0.0", false},
		{"with numbers", "feature123", false},
		{"with dashes", "my-feature-branch", false},
		{"with underscores", "my_feature_branch", false},
		{"with dots", "release.1.0", false},

		// Invalid cases
		{"empty branch", "", true},
		{"starts with dash", "-malicious", true},
		{"command injection semicolon", "main; rm -rf /", true},
		{"command injection pipe", "main | cat /etc/passwd", true},
		{"command injection ampersand", "main && curl evil.com", true},
		{"command injection backtick", "main`whoami`", true},
		{"command injection dollar", "main$(whoami)", true},
		{"special chars", "feature@evil", true},
		{"spaces", "my branch", true},
		{"newline", "main\nmalicious", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBranchName(tt.branch)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBranchName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateProjectName(t *testing.T) {
	tests := []struct {
		name    string
		project string
		wantErr bool
	}{
		// Valid cases
		{"simple name", "myproject", false},
		{"with dash", "my-project", false},
		{"with underscore", "my_project", false},
		{"with numbers", "project123", false},
		{"mixed case", "MyProject", false},
		{"all caps", "MYPROJECT", false},

		// Invalid cases
		{"empty name", "", true},
		{"starts with dash", "-project", true},
		{"starts with dot", ".project", true},
		{"with slash", "my/project", true},
		{"with space", "my project", true},
		{"with @", "my@project", true},
		{"with special chars", "project!", true},
		{"command injection", "project; rm -rf /", true},
		{"path traversal", "../etc/passwd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProjectName(tt.project)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProjectName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSanitizePathForSymlink(t *testing.T) {
	// Create temporary directory structure for testing
	tmpDir := t.TempDir()
	baseDir := filepath.Join(tmpDir, "base")
	targetDir := filepath.Join(baseDir, "target")
	outsideDir := filepath.Join(tmpDir, "outside")

	// Create directories
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("Failed to create test directories: %v", err)
	}
	if err := os.MkdirAll(outsideDir, 0755); err != nil {
		t.Fatalf("Failed to create test directories: %v", err)
	}

	tests := []struct {
		name    string
		base    string
		target  string
		wantErr bool
	}{
		// Valid cases
		{"target within base", baseDir, targetDir, false},
		{"same directory", baseDir, baseDir, false},

		// Path traversal attempts
		{"target outside base", baseDir, outsideDir, true},
		{"explicit traversal", baseDir, filepath.Join(baseDir, "../outside"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SanitizePathForSymlink(tt.base, tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("SanitizePathForSymlink() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		// Valid cases
		{"absolute path", "/var/www/project", "/var/www/project", false},
		{"absolute path with trailing slash", "/var/www/project/", "/var/www/project", false},
		{"absolute path with ./ elements", "/var/www/./project", "/var/www/project", false},

		// Invalid cases
		{"relative path", "var/www/project", "", true},
		{"relative path with dot", "./project", "", true},
		{"path traversal", "/var/www/../../../etc/passwd", "", true},
		{"path with .. in middle", "/var/../etc/passwd", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SanitizePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("SanitizePath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("SanitizePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Benchmark tests
func BenchmarkValidateGitURL(b *testing.B) {
	url := "https://github.com/user/repo.git"
	for i := 0; i < b.N; i++ {
		_ = ValidateGitURL(url)
	}
}

func BenchmarkValidateBranchName(b *testing.B) {
	branch := "feature/my-feature"
	for i := 0; i < b.N; i++ {
		_ = ValidateBranchName(branch)
	}
}

func BenchmarkValidateProjectName(b *testing.B) {
	name := "my-project"
	for i := 0; i < b.N; i++ {
		_ = ValidateProjectName(name)
	}
}
