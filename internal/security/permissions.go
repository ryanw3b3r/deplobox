package security

import (
	"fmt"
	"os"
)

const (
	// PermConfigFile is for configuration files containing sensitive data.
	// rw-r----- (0640): owner can read/write, group can read, others have no access.
	PermConfigFile os.FileMode = 0640

	// PermLogFile is for log files that may contain deployment information.
	// rw-r----- (0640): owner can read/write, group can read, others have no access.
	PermLogFile os.FileMode = 0640

	// PermDBFile is for database files containing deployment history.
	// rw-r----- (0640): owner can read/write, group can read, others have no access.
	PermDBFile os.FileMode = 0640

	// PermExecutable is for executable binaries.
	// rwxr-x--- (0750): owner can read/write/execute, group can read/execute, others have no access.
	PermExecutable os.FileMode = 0750

	// PermDirectory is for standard directories.
	// rwxr-x--- (0750): owner can read/write/execute, group can read/execute, others have no access.
	PermDirectory os.FileMode = 0750

	// PermSharedDir is for shared directories that need group write access.
	// rwxrwx--- (0770): owner and group have full access, others have no access.
	PermSharedDir os.FileMode = 0770

	// PermSSHKey is for private SSH keys.
	// rw------- (0600): only owner can read/write, no one else has access.
	PermSSHKey os.FileMode = 0600

	// PermPublicFile is for public files that can be read by anyone.
	// rw-r--r-- (0644): owner can read/write, group and others can read.
	PermPublicFile os.FileMode = 0644
)

// CreateSecureFile creates a new file with secure permissions.
// If the file exists, it will be truncated.
// Returns an error if the file cannot be created with the specified permissions.
func CreateSecureFile(path string, perm os.FileMode) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return nil, fmt.Errorf("failed to create secure file: %w", err)
	}

	// Explicitly set permissions to bypass umask
	if err := os.Chmod(path, perm); err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to set file permissions: %w", err)
	}

	return file, nil
}

// CreateSecureDir creates a new directory with secure permissions.
// If the directory already exists, it updates the permissions.
// Creates parent directories as needed.
func CreateSecureDir(path string, perm os.FileMode) error {
	if err := os.MkdirAll(path, perm); err != nil {
		return fmt.Errorf("failed to create secure directory: %w", err)
	}

	// Ensure permissions are set correctly (MkdirAll may use umask)
	if err := os.Chmod(path, perm); err != nil {
		return fmt.Errorf("failed to set directory permissions: %w", err)
	}

	return nil
}

// EnsureSecurePermissions checks if a file has the expected permissions.
// Returns an error if permissions are too permissive.
func EnsureSecurePermissions(path string, expectedPerm os.FileMode) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	actualPerm := info.Mode().Perm()

	// Check if actual permissions are more permissive than expected
	if actualPerm&^expectedPerm != 0 {
		return fmt.Errorf("file %s has too permissive permissions: %04o (expected: %04o)",
			path, actualPerm, expectedPerm)
	}

	return nil
}

// FixFilePermissions sets the correct permissions on a file.
// Use this to fix permissions on existing files.
func FixFilePermissions(path string, perm os.FileMode) error {
	if err := os.Chmod(path, perm); err != nil {
		return fmt.Errorf("failed to fix file permissions: %w", err)
	}
	return nil
}

// IsWorldReadable checks if a file is readable by others.
// Returns true if the file has world-readable permissions (e.g., 0644, 0664).
func IsWorldReadable(perm os.FileMode) bool {
	return perm&0004 != 0
}

// IsWorldWritable checks if a file is writable by others.
// Returns true if the file has world-writable permissions (e.g., 0666, 0777).
func IsWorldWritable(perm os.FileMode) bool {
	return perm&0002 != 0
}

// ValidateSecurePermissions validates that a file does not have world-readable
// or world-writable permissions for sensitive files.
func ValidateSecurePermissions(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	perm := info.Mode().Perm()

	if IsWorldReadable(perm) {
		return fmt.Errorf("file %s is world-readable (%04o), which is insecure for sensitive data", path, perm)
	}

	if IsWorldWritable(perm) {
		return fmt.Errorf("file %s is world-writable (%04o), which is a serious security risk", path, perm)
	}

	return nil
}
