package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// UpdateSymlinkAtomic atomically updates a symlink to point to a new target.
// This uses the "create temp, then rename" pattern for zero-downtime updates.
//
// Steps:
// 1. Create a temporary symlink with .tmp suffix
// 2. Atomically rename it to the final name
//
// This ensures that the symlink is always valid and there's no moment
// when it points to nothing or is partially updated.
func UpdateSymlinkAtomic(linkPath, targetPath string) error {
	// Create temp symlink path
	tmpLink := linkPath + ".tmp"

	// Remove temp link if it exists from a previous failed attempt
	_ = os.Remove(tmpLink)

	// Create new symlink with temp name
	if err := os.Symlink(targetPath, tmpLink); err != nil {
		return fmt.Errorf("failed to create temporary symlink: %w", err)
	}

	// Atomically replace the old symlink with the new one
	// On Unix, this is atomic. On Windows, it may fail if the target exists.
	if err := os.Rename(tmpLink, linkPath); err != nil {
		// Clean up temp link on failure
		_ = os.Remove(tmpLink)
		return fmt.Errorf("failed to rename symlink atomically: %w", err)
	}

	return nil
}

// CreateSymlink creates a symlink, removing any existing file/link at that path.
// This is NOT atomic - use UpdateSymlinkAtomic for zero-downtime updates.
func CreateSymlink(linkPath, targetPath string) error {
	// Remove existing link/file if present
	_ = os.Remove(linkPath)

	// Create new symlink
	if err := os.Symlink(targetPath, linkPath); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	return nil
}

// IsSymlink checks if a path is a symlink.
func IsSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

// ReadSymlink reads the target of a symlink.
// Returns an error if the path is not a symlink or cannot be read.
func ReadSymlink(path string) (string, error) {
	target, err := os.Readlink(path)
	if err != nil {
		return "", fmt.Errorf("failed to read symlink: %w", err)
	}
	return target, nil
}

// ResolveSymlink resolves a symlink to its final target.
// If the path is not a symlink, returns the path itself.
// Follows the entire chain of symlinks to the final destination.
func ResolveSymlink(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve symlink: %w", err)
	}
	return resolved, nil
}

// SymlinkExists checks if a symlink exists at the given path.
// Returns true only if the path is a symlink (not a regular file).
func SymlinkExists(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

// SymlinkTarget reads the target of a symlink without resolving it fully.
// This returns the immediate target, not the final destination if there's a chain.
func SymlinkTarget(path string) (string, error) {
	if !IsSymlink(path) {
		return "", fmt.Errorf("path is not a symlink: %s", path)
	}

	target, err := os.Readlink(path)
	if err != nil {
		return "", fmt.Errorf("failed to read symlink target: %w", err)
	}

	return target, nil
}

// ValidateSymlink checks if a symlink exists and points to an existing target.
// Returns an error if:
// - The path is not a symlink
// - The symlink is broken (target doesn't exist)
func ValidateSymlink(path string) error {
	if !IsSymlink(path) {
		return fmt.Errorf("path is not a symlink: %s", path)
	}

	target, err := ResolveSymlink(path)
	if err != nil {
		return fmt.Errorf("symlink is broken: %w", err)
	}

	if _, err := os.Stat(target); err != nil {
		return fmt.Errorf("symlink target does not exist: %s", target)
	}

	return nil
}
