package security

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	// Safe patterns for validation
	gitURLPattern  = regexp.MustCompile(`^https://github\.com/[a-zA-Z0-9_-]+/[a-zA-Z0-9_.-]+(?:\.git)?$`)
	branchPattern  = regexp.MustCompile(`^[a-zA-Z0-9/_.-]+$`)
	projectPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
)

// ValidateGitURL ensures URL is safe for git clone operations.
// Only HTTPS GitHub URLs are allowed to prevent command injection.
func ValidateGitURL(rawURL string) error {
	// Parse URL
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Only HTTPS GitHub URLs
	if u.Scheme != "https" || u.Host != "github.com" {
		return fmt.Errorf("only GitHub HTTPS URLs allowed, got %s://%s", u.Scheme, u.Host)
	}

	// Match safe pattern to prevent injection
	if !gitURLPattern.MatchString(rawURL) {
		return fmt.Errorf("URL contains invalid characters or format")
	}

	return nil
}

// ValidateBranchName ensures branch name is safe for git operations.
// Prevents command injection through branch names.
func ValidateBranchName(branch string) error {
	if branch == "" {
		return fmt.Errorf("branch name cannot be empty")
	}
	if strings.HasPrefix(branch, "-") {
		return fmt.Errorf("branch name cannot start with '-'")
	}
	if !branchPattern.MatchString(branch) {
		return fmt.Errorf("branch name contains invalid characters")
	}
	return nil
}

// ValidateProjectName ensures project name is safe for use in paths and URLs.
func ValidateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	if strings.HasPrefix(name, "-") || strings.HasPrefix(name, ".") {
		return fmt.Errorf("project name cannot start with '-' or '.'")
	}
	if !projectPattern.MatchString(name) {
		return fmt.Errorf("project name contains invalid characters (only a-z, A-Z, 0-9, _, - allowed)")
	}
	return nil
}

// SanitizePathForSymlink prevents path traversal attacks when creating symlinks.
// Ensures target path is within the base directory.
func SanitizePathForSymlink(basePath, targetPath string) (string, error) {
	// Resolve to absolute paths
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base path: %w", err)
	}

	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve target path: %w", err)
	}

	// Evaluate symlinks to get canonical paths
	cleanBase, err := filepath.EvalSymlinks(absBase)
	if err != nil {
		return "", fmt.Errorf("failed to evaluate base path symlinks: %w", err)
	}

	cleanTarget, err := filepath.EvalSymlinks(absTarget)
	if err != nil {
		return "", fmt.Errorf("failed to evaluate target path symlinks: %w", err)
	}

	// Ensure target is within base
	relPath, err := filepath.Rel(cleanBase, cleanTarget)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return "", fmt.Errorf("path traversal detected: target '%s' is outside base '%s'", cleanTarget, cleanBase)
	}

	return cleanTarget, nil
}

// SanitizePath ensures a path is absolute and doesn't contain traversal attempts.
// This is used for general path validation beyond symlinks.
func SanitizePath(path string) (string, error) {
	// Must be absolute
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("path must be absolute: %s", path)
	}

	// Check for .. before cleaning (filepath.Clean removes them)
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("path contains traversal elements: %s", path)
	}

	// Clean the path to remove ./ elements
	cleaned := filepath.Clean(path)

	return cleaned, nil
}
