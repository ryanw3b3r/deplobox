package security

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPermissionConstants(t *testing.T) {
	tests := []struct {
		name     string
		perm     os.FileMode
		expected os.FileMode
	}{
		{"PermConfigFile", PermConfigFile, 0640},
		{"PermLogFile", PermLogFile, 0640},
		{"PermDBFile", PermDBFile, 0640},
		{"PermExecutable", PermExecutable, 0750},
		{"PermDirectory", PermDirectory, 0750},
		{"PermSharedDir", PermSharedDir, 0770},
		{"PermSSHKey", PermSSHKey, 0600},
		{"PermPublicFile", PermPublicFile, 0644},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.perm != tt.expected {
				t.Errorf("%s = %04o, want %04o", tt.name, tt.perm, tt.expected)
			}
		})
	}
}

func TestCreateSecureFile(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		filename string
		perm     os.FileMode
		wantErr  bool
	}{
		{"create config file", "config.yaml", PermConfigFile, false},
		{"create log file", "app.log", PermLogFile, false},
		{"create db file", "data.db", PermDBFile, false},
		{"create ssh key", "id_rsa", PermSSHKey, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tt.filename)
			file, err := CreateSecureFile(path, tt.perm)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateSecureFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			defer file.Close()

			// Verify file exists
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("File was not created: %v", err)
			}

			// Verify permissions
			actualPerm := info.Mode().Perm()
			if actualPerm != tt.perm {
				t.Errorf("File permissions = %04o, want %04o", actualPerm, tt.perm)
			}
		})
	}
}

func TestCreateSecureDir(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		dirname string
		perm    os.FileMode
		wantErr bool
	}{
		{"create standard dir", "mydir", PermDirectory, false},
		{"create shared dir", "shared", PermSharedDir, false},
		{"create nested dir", "parent/child/grandchild", PermDirectory, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(tmpDir, tt.dirname)
			err := CreateSecureDir(path, tt.perm)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateSecureDir() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}

			// Verify directory exists
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("Directory was not created: %v", err)
			}
			if !info.IsDir() {
				t.Fatalf("Created path is not a directory")
			}

			// Verify permissions
			actualPerm := info.Mode().Perm()
			if actualPerm != tt.perm {
				t.Errorf("Directory permissions = %04o, want %04o", actualPerm, tt.perm)
			}
		})
	}
}

func TestEnsureSecurePermissions(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file with specific permissions
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0640); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name         string
		path         string
		expectedPerm os.FileMode
		wantErr      bool
	}{
		{"correct permissions", testFile, 0640, false},
		{"more restrictive allowed", testFile, 0644, false}, // 640 is more restrictive than 644
		{"nonexistent file", filepath.Join(tmpDir, "missing.txt"), 0640, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EnsureSecurePermissions(tt.path, tt.expectedPerm)
			if (err != nil) != tt.wantErr {
				t.Errorf("EnsureSecurePermissions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFixFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create file with wrong permissions
	if err := os.WriteFile(testFile, []byte("test"), 0666); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Force permissions to 0666 (umask might have changed them)
	if err := os.Chmod(testFile, 0666); err != nil {
		t.Fatalf("Failed to set initial permissions: %v", err)
	}

	// Fix permissions
	err := FixFilePermissions(testFile, 0640)
	if err != nil {
		t.Fatalf("FixFilePermissions() failed: %v", err)
	}

	// Verify fixed permissions
	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	if info.Mode().Perm() != 0640 {
		t.Errorf("File permissions = %04o, want 0640", info.Mode().Perm())
	}
}

func TestIsWorldReadable(t *testing.T) {
	tests := []struct {
		name string
		perm os.FileMode
		want bool
	}{
		{"0644 is world readable", 0644, true},
		{"0664 is world readable", 0664, true},
		{"0666 is world readable", 0666, true},
		{"0640 is not world readable", 0640, false},
		{"0600 is not world readable", 0600, false},
		{"0660 is not world readable", 0660, false},
		{"0700 is not world readable", 0700, false},
		{"0755 is world readable", 0755, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsWorldReadable(tt.perm)
			if got != tt.want {
				t.Errorf("IsWorldReadable(%04o) = %v, want %v", tt.perm, got, tt.want)
			}
		})
	}
}

func TestIsWorldWritable(t *testing.T) {
	tests := []struct {
		name string
		perm os.FileMode
		want bool
	}{
		{"0666 is world writable", 0666, true},
		{"0777 is world writable", 0777, true},
		{"0662 is world writable", 0662, true},
		{"0664 is not world writable", 0664, false},
		{"0644 is not world writable", 0644, false},
		{"0600 is not world writable", 0600, false},
		{"0640 is not world writable", 0640, false},
		{"0755 is not world writable", 0755, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsWorldWritable(tt.perm)
			if got != tt.want {
				t.Errorf("IsWorldWritable(%04o) = %v, want %v", tt.perm, got, tt.want)
			}
		})
	}
}

func TestValidateSecurePermissions(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		perm    os.FileMode
		wantErr bool
	}{
		{"secure 0600", 0600, false},
		{"secure 0640", 0640, false},
		{"secure 0660", 0660, false},
		{"secure 0700", 0700, false},
		{"world readable 0644", 0644, true},
		{"world readable 0664", 0664, true},
		{"world writable 0666", 0666, true},
		{"world writable 0777", 0777, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, "test-"+tt.name+".txt")
			if err := os.WriteFile(testFile, []byte("test"), tt.perm); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			err := ValidateSecurePermissions(testFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSecurePermissions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSecurePermissions_NonexistentFile(t *testing.T) {
	err := ValidateSecurePermissions("/nonexistent/file.txt")
	if err == nil {
		t.Errorf("ValidateSecurePermissions() should fail for nonexistent file")
	}
}

func TestCreateSecureFile_Overwrite(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create file with content
	if err := os.WriteFile(testFile, []byte("original content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create secure file (should truncate)
	file, err := CreateSecureFile(testFile, PermConfigFile)
	if err != nil {
		t.Fatalf("CreateSecureFile() failed: %v", err)
	}

	// Write new content
	if _, err := file.WriteString("new content"); err != nil {
		file.Close()
		t.Fatalf("Failed to write to file: %v", err)
	}
	file.Close()

	// Verify content was overwritten
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(content) != "new content" {
		t.Errorf("File content = %q, want %q", string(content), "new content")
	}

	// Verify permissions (may vary due to umask, but should be at least as restrictive)
	info, _ := os.Stat(testFile)
	actualPerm := info.Mode().Perm()
	// Check that it's not world-readable (most important security check)
	if actualPerm&0004 != 0 {
		t.Errorf("File is world-readable (%04o), should not be", actualPerm)
	}
}
