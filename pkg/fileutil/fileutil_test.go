package fileutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Tests for search.go

func TestSearchPaths(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	if err := os.WriteFile(file1, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name    string
		paths   []string
		want    string
		wantErr bool
	}{
		{
			"finds first existing file",
			[]string{file1, file2},
			file1,
			false,
		},
		{
			"returns error when no files exist",
			[]string{file2, filepath.Join(tmpDir, "nonexistent.txt")},
			"",
			true,
		},
		{
			"handles empty path list",
			[]string{},
			"",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SearchPaths(tt.paths)
			if (err != nil) != tt.wantErr {
				t.Errorf("SearchPaths() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("SearchPaths() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSearchPathsOptional(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	file1 := filepath.Join(tmpDir, "file1.txt")
	if err := os.WriteFile(file1, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name  string
		paths []string
		want  string
	}{
		{
			"finds existing file",
			[]string{file1},
			file1,
		},
		{
			"returns empty string when not found",
			[]string{filepath.Join(tmpDir, "nonexistent.txt")},
			"",
		},
		{
			"handles empty path list",
			[]string{},
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SearchPathsOptional(tt.paths)
			if got != tt.want {
				t.Errorf("SearchPathsOptional() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultConfigPaths(t *testing.T) {
	paths := DefaultConfigPaths("test.yaml")

	if len(paths) != 3 {
		t.Errorf("DefaultConfigPaths() returned %d paths, want 3", len(paths))
	}

	// Check that paths contain the filename
	for i, path := range paths {
		if !strings.Contains(path, "test.yaml") {
			t.Errorf("DefaultConfigPaths()[%d] = %v, should contain 'test.yaml'", i, path)
		}
	}

	// Check that the system path is /etc/deplobox/...
	if !strings.HasPrefix(paths[2], "/etc/deplobox") {
		t.Errorf("DefaultConfigPaths()[2] should start with /etc/deplobox, got %v", paths[2])
	}
}

func TestFindConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create config in current directory
	configFile := filepath.Join(tmpDir, "projects.yaml")
	if err := os.WriteFile(configFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Test finding existing config
	t.Run("finds existing config", func(t *testing.T) {
		paths := []string{configFile, filepath.Join(tmpDir, "config", "projects.yaml")}
		found, err := SearchPaths(paths)
		if err != nil {
			t.Errorf("SearchPaths() error = %v", err)
		}
		if found != configFile {
			t.Errorf("SearchPaths() = %v, want %v", found, configFile)
		}
	})

	// Test not finding config
	t.Run("returns error when not found", func(t *testing.T) {
		paths := []string{filepath.Join(tmpDir, "nonexistent.yaml")}
		_, err := SearchPaths(paths)
		if err == nil {
			t.Error("SearchPaths() should return error when config not found")
		}
	})
}

func TestFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create test directory
	testDir := filepath.Join(tmpDir, "testdir")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"existing file", testFile, true},
		{"nonexistent file", filepath.Join(tmpDir, "nonexistent.txt"), false},
		{"directory", testDir, false}, // Directories return false
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FileExists(tt.path)
			if got != tt.want {
				t.Errorf("FileExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDirExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test directory
	testDir := filepath.Join(tmpDir, "testdir")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"existing directory", testDir, true},
		{"nonexistent directory", filepath.Join(tmpDir, "nonexistent"), false},
		{"file", testFile, false}, // Files return false
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DirExists(tt.path)
			if got != tt.want {
				t.Errorf("DirExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPathExists(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create test directory
	testDir := filepath.Join(tmpDir, "testdir")
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"existing file", testFile, true},
		{"existing directory", testDir, true},
		{"nonexistent path", filepath.Join(tmpDir, "nonexistent"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PathExists(tt.path)
			if got != tt.want {
				t.Errorf("PathExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Tests for symlink.go

func TestUpdateSymlinkAtomic(t *testing.T) {
	tmpDir := t.TempDir()

	// Create target directories
	target1 := filepath.Join(tmpDir, "target1")
	target2 := filepath.Join(tmpDir, "target2")
	if err := os.Mkdir(target1, 0755); err != nil {
		t.Fatalf("Failed to create target1: %v", err)
	}
	if err := os.Mkdir(target2, 0755); err != nil {
		t.Fatalf("Failed to create target2: %v", err)
	}

	linkPath := filepath.Join(tmpDir, "current")

	// Test creating new symlink
	t.Run("create new symlink", func(t *testing.T) {
		err := UpdateSymlinkAtomic(linkPath, target1)
		if err != nil {
			t.Fatalf("UpdateSymlinkAtomic() error = %v", err)
		}

		// Verify symlink exists and points to target1
		target, err := os.Readlink(linkPath)
		if err != nil {
			t.Fatalf("Failed to read symlink: %v", err)
		}
		if target != target1 {
			t.Errorf("Symlink points to %v, want %v", target, target1)
		}
	})

	// Test updating existing symlink
	t.Run("update existing symlink", func(t *testing.T) {
		err := UpdateSymlinkAtomic(linkPath, target2)
		if err != nil {
			t.Fatalf("UpdateSymlinkAtomic() error = %v", err)
		}

		// Verify symlink now points to target2
		target, err := os.Readlink(linkPath)
		if err != nil {
			t.Fatalf("Failed to read symlink: %v", err)
		}
		if target != target2 {
			t.Errorf("Symlink points to %v, want %v", target, target2)
		}
	})
}

func TestCreateSymlink(t *testing.T) {
	tmpDir := t.TempDir()

	target := filepath.Join(tmpDir, "target")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	linkPath := filepath.Join(tmpDir, "link")

	err := CreateSymlink(linkPath, target)
	if err != nil {
		t.Fatalf("CreateSymlink() error = %v", err)
	}

	// Verify symlink was created
	if !IsSymlink(linkPath) {
		t.Error("CreateSymlink() did not create a symlink")
	}

	// Verify target
	readTarget, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}
	if readTarget != target {
		t.Errorf("Symlink points to %v, want %v", readTarget, target)
	}
}

func TestIsSymlink(t *testing.T) {
	tmpDir := t.TempDir()

	// Create regular file
	regularFile := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(regularFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create regular file: %v", err)
	}

	// Create symlink
	target := filepath.Join(tmpDir, "target")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}
	link := filepath.Join(tmpDir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"symlink", link, true},
		{"regular file", regularFile, false},
		{"nonexistent", filepath.Join(tmpDir, "nonexistent"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSymlink(tt.path)
			if got != tt.want {
				t.Errorf("IsSymlink() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReadSymlink(t *testing.T) {
	tmpDir := t.TempDir()

	target := filepath.Join(tmpDir, "target")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	link := filepath.Join(tmpDir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	readTarget, err := ReadSymlink(link)
	if err != nil {
		t.Fatalf("ReadSymlink() error = %v", err)
	}

	if readTarget != target {
		t.Errorf("ReadSymlink() = %v, want %v", readTarget, target)
	}
}

func TestResolveSymlink(t *testing.T) {
	tmpDir := t.TempDir()

	target := filepath.Join(tmpDir, "target")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	link := filepath.Join(tmpDir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	resolved, err := ResolveSymlink(link)
	if err != nil {
		t.Fatalf("ResolveSymlink() error = %v", err)
	}

	// On macOS, symlinks may resolve with /private prefix
	// Check that the resolved path ends with the same suffix as target
	if !strings.HasSuffix(resolved, filepath.Base(target)) {
		t.Errorf("ResolveSymlink() = %v, should end with %v", resolved, filepath.Base(target))
	}
}

func TestValidateSymlink(t *testing.T) {
	tmpDir := t.TempDir()

	// Create valid symlink
	target := filepath.Join(tmpDir, "target")
	if err := os.Mkdir(target, 0755); err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}
	validLink := filepath.Join(tmpDir, "valid-link")
	if err := os.Symlink(target, validLink); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Create broken symlink
	brokenLink := filepath.Join(tmpDir, "broken-link")
	if err := os.Symlink(filepath.Join(tmpDir, "nonexistent"), brokenLink); err != nil {
		t.Fatalf("Failed to create broken symlink: %v", err)
	}

	// Create regular file
	regularFile := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(regularFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create regular file: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid symlink", validLink, false},
		{"broken symlink", brokenLink, true},
		{"regular file", regularFile, true},
		{"nonexistent", filepath.Join(tmpDir, "nonexistent"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSymlink(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSymlink() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
