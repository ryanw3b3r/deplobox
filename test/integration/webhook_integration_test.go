package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"deplobox/internal/history"
	"deplobox/internal/project"
	"deplobox/internal/server"
)

// TestWebhookSignatureValidation tests signature verification in webhook requests
func TestWebhookSignatureValidation(t *testing.T) {
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "sig-test")

	// Create minimal project structure
	os.MkdirAll(filepath.Join(projectPath, "releases"), 0755)
	os.MkdirAll(filepath.Join(projectPath, "shared"), 0755)

	// Create initial release with proper git repository
	initialRelease := filepath.Join(projectPath, "releases", "20250101000000")
	os.MkdirAll(initialRelease, 0755)
	setupTestGitRepo(t, initialRelease)

	currentLink := filepath.Join(projectPath, "current")
	os.Symlink(initialRelease, currentLink)

	secret := "test-secret-at-least-32-chars-long-for-signature-validation"
	testProject := &project.Project{
		Name:              "sig-test",
		Path:              projectPath,
		Secret:            secret,
		Branch:            "main",
		PullTimeout:       60,
		PostDeployTimeout: 300,
		PostDeploy:        []interface{}{},
	}

	registry := project.NewRegistry(map[string]*project.Project{
		"sig-test": testProject,
	})

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	srv := server.NewServer(registry, nil, logger, true)

	payload := []byte(`{"ref":"refs/heads/main","after":"abc123"}`)

	tests := []struct {
		name           string
		signature      string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "valid signature",
			signature:      server.MakeTestSignature(payload, secret),
			expectedStatus: http.StatusAccepted, // Deployment accepted and runs async
			expectedError:  "", // No signature error
		},
		{
			name:           "invalid signature",
			signature:      server.MakeTestSignature(payload, "wrong-secret-32-chars-long-wrongwrong"),
			expectedStatus: http.StatusForbidden,
			expectedError:  "Invalid signature",
		},
		{
			name:           "missing signature",
			signature:      "",
			expectedStatus: http.StatusForbidden,
			expectedError:  "Invalid signature",
		},
		{
			name:           "malformed signature",
			signature:      "invalid-format",
			expectedStatus: http.StatusForbidden,
			expectedError:  "Invalid signature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/in/sig-test", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-GitHub-Event", "push")

			if tt.signature != "" {
				req.Header.Set("X-Hub-Signature-256", tt.signature)
			}

			rr := httptest.NewRecorder()
			srv.Router().ServeHTTP(rr, req)

			if tt.expectedError != "" {
				// Expecting an error
				var response map[string]string
				json.Unmarshal(rr.Body.Bytes(), &response)

				if response["error"] != tt.expectedError {
					t.Errorf("Expected error '%s', got '%s'", tt.expectedError, response["error"])
				}
			}

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Wait for any async deployment to complete before next test
			srv.WaitForDeployments()
		})
	}
}

// TestWebhookHistoryRecording tests that deployments are recorded in history
func TestWebhookHistoryRecording(t *testing.T) {
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "history-test")

	// Create minimal project structure
	os.MkdirAll(filepath.Join(projectPath, "releases"), 0755)
	os.MkdirAll(filepath.Join(projectPath, "shared"), 0755)

	// Create initial release with proper git repository
	initialRelease := filepath.Join(projectPath, "releases", "20250101000000")
	os.MkdirAll(initialRelease, 0755)
	setupTestGitRepo(t, initialRelease)

	currentLink := filepath.Join(projectPath, "current")
	os.Symlink(initialRelease, currentLink)

	secret := "history-test-secret-at-least-32-chars-long-here"
	testProject := &project.Project{
		Name:              "history-test",
		Path:              projectPath,
		Secret:            secret,
		Branch:            "main",
		PullTimeout:       60,
		PostDeployTimeout: 300,
		PostDeploy:        []interface{}{},
	}

	registry := project.NewRegistry(map[string]*project.Project{
		"history-test": testProject,
	})

	dbPath := filepath.Join(tmpDir, "test.db")
	hist, err := history.NewHistory(dbPath)
	if err != nil {
		t.Fatalf("Failed to create history: %v", err)
	}
	defer hist.Close()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	srv := server.NewServer(registry, hist, logger, false)

	// Test that failed deployment is recorded
	payload := []byte(`{"ref":"refs/heads/main","after":"abc123"}`)
	signature := server.MakeTestSignature(payload, secret)

	req := httptest.NewRequest("POST", "/in/history-test", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", signature)

	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	// Webhook now returns 202 Accepted and processes async
	if rr.Code != http.StatusAccepted {
		t.Errorf("Expected status %d, got %d", http.StatusAccepted, rr.Code)
	}

	// Wait for async deployment to complete
	srv.WaitForDeployments()

	// Verify deployment was recorded
	latest, err := hist.GetLatestDeployment(context.Background(), "history-test")
	if err != nil {
		t.Fatalf("Failed to get latest deployment: %v", err)
	}

	if latest == nil {
		t.Error("Expected deployment to be recorded in history")
	} else {
		if latest.Project != "history-test" {
			t.Errorf("Expected project 'history-test', got '%s'", latest.Project)
		}
		if latest.Branch != "main" {
			t.Errorf("Expected branch 'main', got '%s'", latest.Branch)
		}
		// Status should be success since we set up proper git repos
		if latest.Status != "success" {
			t.Errorf("Expected status 'success', got '%s'", latest.Status)
		}
	}
}

// TestConcurrentDeploymentLocking tests that only one deployment runs at a time
func TestConcurrentDeploymentLocking(t *testing.T) {
	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "lock-test")

	// Create minimal project structure
	os.MkdirAll(filepath.Join(projectPath, "releases"), 0755)
	os.MkdirAll(filepath.Join(projectPath, "shared"), 0755)

	// Create initial release with proper git repository (not used, but needed for consistency)
	initialRelease := filepath.Join(projectPath, "releases", "20250101000000")
	os.MkdirAll(initialRelease, 0755)
	setupTestGitRepo(t, initialRelease)

	currentLink := filepath.Join(projectPath, "current")
	os.Symlink(initialRelease, currentLink)

	secret := "lock-test-secret-at-least-32-chars-long-here"
	testProject := &project.Project{
		Name:              "lock-test",
		Path:              projectPath,
		Secret:            secret,
		Branch:            "main",
		PullTimeout:       60,
		PostDeployTimeout: 300,
		PostDeploy:        []interface{}{}, // No slow operations
	}

	registry := project.NewRegistry(map[string]*project.Project{
		"lock-test": testProject,
	})

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	srv := server.NewServer(registry, nil, logger, true) // Test mode

	payload := []byte(`{"ref":"refs/heads/main","after":"abc123"}`)
	signature := server.MakeTestSignature(payload, secret)

	// Manually acquire lock
	if !srv.LockManager.TryLock("lock-test") {
		t.Fatal("Failed to acquire initial lock")
	}
	defer srv.LockManager.Unlock("lock-test")

	// Attempt deployment while lock is held
	req := httptest.NewRequest("POST", "/in/lock-test", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", signature)

	rr := httptest.NewRecorder()
	srv.Router().ServeHTTP(rr, req)

	// Verify request was rejected with 429
	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", rr.Code)
	}

	var response map[string]string
	json.Unmarshal(rr.Body.Bytes(), &response)

	if response["error"] != "Deployment already in progress" {
		t.Errorf("Expected 'Deployment already in progress' error, got '%s'", response["error"])
	}
}

// TestWebhookProjectValidation tests project name validation
func TestWebhookProjectValidation(t *testing.T) {
	tmpDir := t.TempDir()

	registry := project.NewRegistry(map[string]*project.Project{})
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	srv := server.NewServer(registry, nil, logger, true)

	_ = tmpDir

	tests := []struct {
		name           string
		projectName    string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "unknown project",
			projectName:    "unknown-project",
			expectedStatus: http.StatusNotFound,
			expectedError:  "Unknown project",
		},
		{
			name:           "invalid project name with slashes",
			projectName:    "../../../etc/passwd",
			expectedStatus: http.StatusNotFound, // Router doesn't match this path
			expectedError:  "",
		},
		{
			name:           "invalid project name with shell chars",
			projectName:    "project; rm -rf /",
			expectedStatus: http.StatusNotFound, // Router doesn't match this path
			expectedError:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := []byte(`{"ref":"refs/heads/main"}`)

			// Create request - use direct handler invocation for malformed URLs
			req := httptest.NewRequest("POST", "/in/placeholder", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-GitHub-Event", "push")

			// Manually set the URL path with the test project name
			// This allows us to test validation without httptest.NewRequest parsing the URL
			req.URL.Path = "/in/" + tt.projectName

			rr := httptest.NewRecorder()
			srv.Router().ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.expectedError != "" {
				var response map[string]string
				json.Unmarshal(rr.Body.Bytes(), &response)

				if response["error"] != tt.expectedError {
					t.Errorf("Expected error '%s', got '%s'", tt.expectedError, response["error"])
				}
			}

			_ = payload
		})
	}
}
