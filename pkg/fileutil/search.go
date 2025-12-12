package fileutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// SearchPaths looks for a file in multiple locations.
// Returns the first path where the file exists, or an error if not found.
func SearchPaths(paths []string) (string, error) {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("file not found in any of the search paths: %v", paths)
}

// SearchPathsOptional looks for a file in multiple locations.
// Returns the first path where the file exists, or empty string if not found.
// This is useful when a file is optional and you don't want an error.
func SearchPathsOptional(paths []string) string {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// DefaultConfigPaths returns standard config search paths for a given filename.
// Search order:
// 1. Current directory (./<filename>)
// 2. Config subdirectory (./config/<filename>)
// 3. System-wide config (/etc/deplobox/<filename>)
func DefaultConfigPaths(filename string) []string {
	return []string{
		filepath.Join(".", filename),
		filepath.Join(".", "config", filename),
		filepath.Join("/etc/deplobox", filename),
	}
}

// FindConfig searches for a config file in default locations.
// Returns the path if found, or an error if not found.
func FindConfig(filename string) (string, error) {
	return SearchPaths(DefaultConfigPaths(filename))
}

// FindConfigOptional searches for a config file in default locations.
// Returns the path if found, or empty string if not found.
func FindConfigOptional(filename string) string {
	return SearchPathsOptional(DefaultConfigPaths(filename))
}

// FileExists checks if a file exists and is not a directory.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// DirExists checks if a directory exists.
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// PathExists checks if a path exists (file or directory).
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
