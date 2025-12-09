package integration

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"deplobox/internal/deployment"
	"deplobox/internal/history"
	"deplobox/internal/project"
	"deplobox/internal/server"
	"deplobox/pkg/fileutil"
)

// TestEndToEndDeployment tests the complete deployment workflow
func TestEndToEndDeployment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary directory structure
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "test-project")
	releasesDir := filepath.Join(projectPath, "releases")
	sharedDir := filepath.Join(projectPath, "shared")
	currentLink := filepath.Join(projectPath, "current")

	// Create directories
	if err := os.MkdirAll(releasesDir, 0755); err != nil {
		t.Fatalf("Failed to create releases dir: %v", err)
	}
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatalf("Failed to create shared dir: %v", err)
	}

	// Create initial release (simulating installer)
	initialRelease := filepath.Join(releasesDir, "2025-01-01-00-00-00")
	if err := os.MkdirAll(initialRelease, 0755); err != nil {
		t.Fatalf("Failed to create initial release dir: %v", err)
	}

	// Initialize it as a proper git repository
	if err := setupTestGitRepo(t, initialRelease); err != nil {
		t.Fatalf("Failed to setup initial release git repo: %v", err)
	}

	// Create current symlink pointing to initial release
	if err := os.Symlink(initialRelease, currentLink); err != nil {
		t.Fatalf("Failed to create current symlink: %v", err)
	}

	// Create project configuration
	testProject := &project.Project{
		Name:              "test-project",
		Path:              projectPath,
		Secret:            "test-secret-at-least-32-chars-long-here-for-testing",
		Branch:            "main",
		PullTimeout:       60,
		PostDeployTimeout: 300,
		PostDeploy:        []interface{}{"echo 'Deployment successful'"},
	}

	// Test 1: First deployment creates release
	t.Run("FirstDeployment", func(t *testing.T) {
		payload := map[string]interface{}{
			"ref":   "refs/heads/main",
			"after": "abc123",
		}

		deploy := deployment.NewDeployment(testProject, payload, false)
		response, statusCode := deploy.Execute(context.Background())

		if statusCode != 200 {
			t.Errorf("Expected status 200, got %d: %v", statusCode, response)
		}

		// Verify release was created
		releases, err := os.ReadDir(releasesDir)
		if err != nil {
			t.Fatalf("Failed to read releases dir: %v", err)
		}

		if len(releases) == 0 {
			t.Error("Expected at least one release to be created")
		}

		// Verify current symlink was created
		if _, err := os.Lstat(currentLink); os.IsNotExist(err) {
			t.Error("Current symlink was not created")
		}

		// Verify symlink points to the release
		target, err := fileutil.ResolveSymlink(currentLink)
		if err != nil {
			t.Errorf("Failed to resolve symlink: %v", err)
		}

		if !strings.Contains(target, "releases") {
			t.Errorf("Symlink does not point to releases directory: %s", target)
		}
	})

	// Test 2: Multiple deployments create multiple releases
	t.Run("MultipleDeployments", func(t *testing.T) {
		// Create 3 more deployments
		for i := 0; i < 3; i++ {
			time.Sleep(1 * time.Second) // Ensure different timestamps (format is to the second)

			payload := map[string]interface{}{
				"ref":   "refs/heads/main",
				"after": "def456",
			}

			deploy := deployment.NewDeployment(testProject, payload, false)
			response, statusCode := deploy.Execute(context.Background())

			if statusCode != 200 {
				t.Errorf("Deployment %d failed with status %d: %v", i+1, statusCode, response)
			}
		}

		// Verify we have at most DefaultKeepReleases (5) releases
		releases, err := os.ReadDir(releasesDir)
		if err != nil {
			t.Fatalf("Failed to read releases dir: %v", err)
		}

		if len(releases) > deployment.DefaultKeepReleases {
			t.Errorf("Expected at most %d releases, got %d", deployment.DefaultKeepReleases, len(releases))
		}

		// Verify current symlink still points to latest release
		if len(releases) > 0 {
			target, err := fileutil.ResolveSymlink(currentLink)
			if err != nil {
				t.Errorf("Failed to resolve symlink: %v", err)
			}

			// Get latest release name
			latestRelease := releases[len(releases)-1].Name()

			if !strings.HasSuffix(target, latestRelease) {
				t.Errorf("Symlink does not point to latest release. Expected suffix %s, got %s", latestRelease, target)
			}
		}
	})

	// Test 3: Shared files are preserved across deployments
	t.Run("SharedFilesPreserved", func(t *testing.T) {
		// Create a file in shared directory
		testFile := filepath.Join(sharedDir, "test.txt")
		testContent := "shared content"
		if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Perform deployment
		payload := map[string]interface{}{
			"ref":   "refs/heads/main",
			"after": "ghi789",
		}

		deploy := deployment.NewDeployment(testProject, payload, false)
		deploy.Execute(context.Background())

		// Verify shared file still exists
		if _, err := os.Stat(testFile); os.IsNotExist(err) {
			t.Error("Shared file was deleted during deployment")
		}

		// Verify content is preserved
		content, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("Failed to read shared file: %v", err)
		}

		if string(content) != testContent {
			t.Errorf("Shared file content changed. Expected %s, got %s", testContent, content)
		}
	})
}

// TestWebhookIntegration tests the full webhook → deployment flow
func TestWebhookIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test environment
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "webhook-project")

	// Create minimal project structure
	if err := os.MkdirAll(filepath.Join(projectPath, "releases"), 0755); err != nil {
		t.Fatalf("Failed to create releases dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectPath, "shared"), 0755); err != nil {
		t.Fatalf("Failed to create shared dir: %v", err)
	}

	// Create test project
	secret := "webhook-test-secret-at-least-32-chars-long-here"
	testProject := &project.Project{
		Name:              "webhook-project",
		Path:              projectPath,
		Secret:            secret,
		Branch:            "main",
		PullTimeout:       60,
		PostDeployTimeout: 300,
		PostDeploy:        []interface{}{},
	}

	// Create registry and history
	registry := project.NewRegistry(map[string]*project.Project{
		"webhook-project": testProject,
	})

	dbPath := filepath.Join(tmpDir, "test.db")
	hist, err := history.NewHistory(dbPath)
	if err != nil {
		t.Fatalf("Failed to create history: %v", err)
	}
	defer hist.Close()

	// Create logger for server
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Create server
	srv := server.NewServer(registry, hist, logger, false)

	// Create initial release for webhook project
	initialRelease := filepath.Join(projectPath, "releases", "2025-01-01-00-00-00")
	if err := os.MkdirAll(initialRelease, 0755); err != nil {
		t.Fatalf("Failed to create initial release dir: %v", err)
	}

	// Initialize it as a proper git repository
	if err := setupTestGitRepo(t, initialRelease); err != nil {
		t.Fatalf("Failed to setup initial release git repo: %v", err)
	}

	currentLink := filepath.Join(projectPath, "current")
	if err := os.Symlink(initialRelease, currentLink); err != nil {
		t.Fatalf("Failed to create current symlink: %v", err)
	}

	// Test webhook request
	payload := []byte(`{"ref":"refs/heads/main","after":"abc123"}`)
	signature := server.MakeTestSignature(payload, secret)

	req := httptest.NewRequest("POST", "/in/webhook-project", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", signature)

	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	// Verify response
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify deployment was recorded in history
	latest, err := hist.GetLatestDeployment(context.Background(), "webhook-project")
	if err != nil {
		t.Fatalf("Failed to get latest deployment: %v", err)
	}

	if latest == nil {
		t.Error("Expected deployment to be recorded in history")
	} else {
		if latest.Project != "webhook-project" {
			t.Errorf("Expected project 'webhook-project', got '%s'", latest.Project)
		}
		if latest.Branch != "main" {
			t.Errorf("Expected branch 'main', got '%s'", latest.Branch)
		}
	}
}

// TestConcurrentDeployments ensures only one deployment runs at a time
func TestConcurrentDeployments(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test environment
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "concurrent-project")

	if err := os.MkdirAll(filepath.Join(projectPath, "releases"), 0755); err != nil {
		t.Fatalf("Failed to create releases dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(projectPath, "shared"), 0755); err != nil {
		t.Fatalf("Failed to create shared dir: %v", err)
	}

	secret := "concurrent-test-secret-at-least-32-chars-long"
	testProject := &project.Project{
		Name:              "concurrent-project",
		Path:              projectPath,
		Secret:            secret,
		Branch:            "main",
		PullTimeout:       60,
		PostDeployTimeout: 300,
		PostDeploy:        []interface{}{"sleep 0.5"}, // Slow deployment
	}

	registry := project.NewRegistry(map[string]*project.Project{
		"concurrent-project": testProject,
	})

	// Create logger for server
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	srv := server.NewServer(registry, nil, logger, true) // Test mode

	// Create initial release
	initialRelease := filepath.Join(projectPath, "releases", "2025-01-01-00-00-00")
	if err := os.MkdirAll(initialRelease, 0755); err != nil {
		t.Fatalf("Failed to create initial release dir: %v", err)
	}

	// Initialize it as a proper git repository
	if err := setupTestGitRepo(t, initialRelease); err != nil {
		t.Fatalf("Failed to setup initial release git repo: %v", err)
	}

	currentLink := filepath.Join(projectPath, "current")
	if err := os.Symlink(initialRelease, currentLink); err != nil {
		t.Fatalf("Failed to create current symlink: %v", err)
	}

	payload := []byte(`{"ref":"refs/heads/main","after":"abc123"}`)
	signature := server.MakeTestSignature(payload, secret)

	// Start first deployment in background
	done := make(chan bool)
	go func() {
		req := httptest.NewRequest("POST", "/in/concurrent-project", strings.NewReader(string(payload)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-GitHub-Event", "push")
		req.Header.Set("X-Hub-Signature-256", signature)

		rr := httptest.NewRecorder()
		srv.Router().ServeHTTP(rr, req)
		done <- true
	}()

	// Give first deployment time to acquire lock
	time.Sleep(100 * time.Millisecond)

	// Attempt second concurrent deployment
	req := httptest.NewRequest("POST", "/in/concurrent-project", strings.NewReader(string(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", signature)

	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	// Verify second deployment was rejected
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", rr.Code)
	}

	var response map[string]string
	json.Unmarshal(rr.Body.Bytes(), &response)

	if response["error"] != "Deployment already in progress" {
		t.Errorf("Expected 'Deployment already in progress' error, got %v", response)
	}

	// Wait for first deployment to complete
	<-done
}

// setupTestGitRepo initializes a minimal git repository for testing
// It creates a bare repository as origin and clones it to the target path
func setupTestGitRepo(t *testing.T, path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}

	// Create a bare repository in a temp location to serve as origin
	// Put it in the grandparent directory to avoid it being listed as a release
	bareRepoPath := filepath.Join(filepath.Dir(filepath.Dir(path)), "origin.git")
	bareCmds := [][]string{
		{"git", "init", "--bare", bareRepoPath},
	}

	for _, cmdParts := range bareCmds {
		cmd := exec.Command(cmdParts[0], cmdParts[1:]...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Logf("Bare repo init failed: %v, output: %s", err, output)
			return err
		}
	}

	// Initialize the working repository
	commands := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@example.com"},
		{"git", "config", "user.name", "Test User"},
		{"sh", "-c", "echo 'test' > README.md"},
		{"git", "add", "README.md"},
		{"git", "commit", "-m", "Initial commit"},
		{"git", "branch", "-M", "main"},
		{"git", "remote", "add", "origin", bareRepoPath},
		{"git", "push", "-u", "origin", "main"},
	}

	for _, cmdParts := range commands {
		cmd := exec.Command(cmdParts[0], cmdParts[1:]...)
		cmd.Dir = path
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Logf("Command %v failed: %v, output: %s", cmdParts, err, output)
			return err
		}
	}

	return nil
}
