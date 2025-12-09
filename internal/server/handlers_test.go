package server

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
)

func setupTestServer(t *testing.T) (*Server, *project.Project) {
	// Create temporary directory with .git
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	// Create test project
	testProject := &project.Project{
		Name:              "test-project",
		Path:              tmpDir,
		Secret:            "test-secret-at-least-32-chars-long-here",
		Branch:            "main",
		PullTimeout:       60,
		PostDeployTimeout: 300,
		PostDeploy:        []interface{}{},
	}

	// Create registry
	registry := project.NewRegistry(map[string]*project.Project{
		"test-project": testProject,
	})

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Create server (test mode - no history)
	server := NewServer(registry, nil, logger, true)

	return server, testProject
}

func TestHandleWebhook_UnknownProject(t *testing.T) {
	server, _ := setupTestServer(t)

	payload := []byte(`{"ref":"refs/heads/main"}`)
	req := httptest.NewRequest("POST", "/in/unknown-project", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "push")

	rr := httptest.NewRecorder()
	server.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rr.Code)
	}

	var response map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &response)

	if response["error"] != "Unknown project" {
		t.Errorf("Expected 'Unknown project' error, got %v", response)
	}
}

func TestHandleWebhook_InvalidSignature(t *testing.T) {
	server, testProject := setupTestServer(t)

	payload := []byte(`{"ref":"refs/heads/main"}`)
	wrongSignature := makeTestSignature(payload, "wrong-secret-32-chars-long-xxxxxxx")

	req := httptest.NewRequest("POST", "/in/test-project", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", wrongSignature)

	rr := httptest.NewRecorder()
	server.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", rr.Code)
	}

	var response map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &response)

	if response["error"] != "Invalid signature" {
		t.Errorf("Expected 'Invalid signature' error, got %v", response)
	}

	_ = testProject
}

func TestHandleWebhook_PayloadTooLarge(t *testing.T) {
	server, _ := setupTestServer(t)

	// Create payload larger than 1MB
	largePayload := make([]byte, MaxPayloadBytes+1)

	req := httptest.NewRequest("POST", "/in/test-project", bytes.NewReader(largePayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "push")

	rr := httptest.NewRecorder()
	server.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413, got %d", rr.Code)
	}
}

func TestHandleWebhook_InvalidContentType(t *testing.T) {
	server, testProject := setupTestServer(t)

	payload := []byte(`{"ref":"refs/heads/main"}`)
	signature := makeTestSignature(payload, testProject.Secret)

	req := httptest.NewRequest("POST", "/in/test-project", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "text/plain") // Wrong content type
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", signature)

	rr := httptest.NewRecorder()
	server.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusUnsupportedMediaType {
		t.Errorf("Expected status 415, got %d", rr.Code)
	}
}

func TestHandleWebhook_NonPushEvent(t *testing.T) {
	server, testProject := setupTestServer(t)

	payload := []byte(`{"ref":"refs/heads/main"}`)
	signature := makeTestSignature(payload, testProject.Secret)

	req := httptest.NewRequest("POST", "/in/test-project", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "pull_request") // Not a push event
	req.Header.Set("X-Hub-Signature-256", signature)

	rr := httptest.NewRecorder()
	server.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &response)

	if response["message"] != "Ignoring non-push event" {
		t.Errorf("Expected 'Ignoring non-push event' message, got %v", response)
	}
}

func TestHandleWebhook_MissingPayload(t *testing.T) {
	server, testProject := setupTestServer(t)

	payload := []byte(`{}`) // Empty JSON object
	signature := makeTestSignature(payload, testProject.Secret)

	req := httptest.NewRequest("POST", "/in/test-project", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", signature)

	rr := httptest.NewRecorder()
	server.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &response)

	if response["message"] != "Missing payload, skipping" {
		t.Errorf("Expected 'Missing payload, skipping' message, got %v", response)
	}
}

func TestHandleWebhook_NonTargetBranch(t *testing.T) {
	server, testProject := setupTestServer(t)

	payload := []byte(`{"ref":"refs/heads/develop"}`) // Not main branch
	signature := makeTestSignature(payload, testProject.Secret)

	req := httptest.NewRequest("POST", "/in/test-project", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", signature)

	rr := httptest.NewRecorder()
	server.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &response)

	if response["message"] != "Not target branch, skipping" {
		t.Errorf("Expected 'Not target branch, skipping' message, got %v", response)
	}
}

func TestHandleWebhook_ConcurrentDeployment(t *testing.T) {
	server, testProject := setupTestServer(t)

	// Acquire lock manually to simulate in-progress deployment
	server.LockManager.TryLock("test-project")
	defer server.LockManager.Unlock("test-project")

	payload := []byte(`{"ref":"refs/heads/main"}`)
	signature := makeTestSignature(payload, testProject.Secret)

	req := httptest.NewRequest("POST", "/in/test-project", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", signature)

	rr := httptest.NewRecorder()
	server.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", rr.Code)
	}

	var response map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &response)

	if response["error"] != "Deployment already in progress" {
		t.Errorf("Expected 'Deployment already in progress' error, got %v", response)
	}
}

func TestHandleHealth(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	server.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &response)

	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", response["status"])
	}

	projects, ok := response["projects"].([]interface{})
	if !ok {
		t.Error("Expected projects to be an array")
	}

	if len(projects) != 1 {
		t.Errorf("Expected 1 project, got %d", len(projects))
	}

	projectCount, ok := response["project_count"].(float64)
	if !ok || projectCount != 1 {
		t.Errorf("Expected project_count 1, got %v", response["project_count"])
	}
}

func TestHandleStatus_UnknownProject(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/status/unknown-project", nil)
	rr := httptest.NewRecorder()

	server.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", rr.Code)
	}
}

func TestHandleStatus_TestMode(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest("GET", "/status/test-project", nil)
	rr := httptest.NewRecorder()

	server.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503 (test mode), got %d", rr.Code)
	}

	var response map[string]string
	_ = json.Unmarshal(rr.Body.Bytes(), &response)

	if response["error"] != "History not available in test mode" {
		t.Errorf("Expected test mode error, got %v", response)
	}
}

func TestHandleStatus_Success(t *testing.T) {
	// Create temporary directory with .git
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	os.Mkdir(gitDir, 0755)

	testProject := &project.Project{
		Name:              "test-project",
		Path:              tmpDir,
		Secret:            "test-secret-at-least-32-chars-long-here",
		Branch:            "main",
		PullTimeout:       60,
		PostDeployTimeout: 300,
		PostDeploy:        []interface{}{},
	}

	registry := project.NewRegistry(map[string]*project.Project{
		"test-project": testProject,
	})

	// Create temporary database
	dbPath := filepath.Join(tmpDir, "test.db")
	hist, err := history.NewHistory(dbPath)
	if err != nil {
		t.Fatalf("Failed to create history: %v", err)
	}
	defer hist.Close()

	// Record a test deployment
	duration := 1.5
	_, err = hist.RecordDeployment(context.Background(), &history.DeploymentRecord{
		Project:         "test-project",
		Branch:          "main",
		Ref:             "refs/heads/main",
		Status:          "success",
		DurationSeconds: &duration,
	})
	if err != nil {
		t.Fatalf("Failed to record deployment: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	server := NewServer(registry, hist, logger, false) // NOT test mode

	req := httptest.NewRequest("GET", "/status/test-project", nil)
	rr := httptest.NewRecorder()

	server.Router().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response map[string]interface{}
	_ = json.Unmarshal(rr.Body.Bytes(), &response)

	if response["project"] != "test-project" {
		t.Errorf("Expected project 'test-project', got %v", response["project"])
	}

	if response["latest_deployment"] == nil {
		t.Error("Expected latest_deployment to be present")
	}

	if response["recent_deployments"] == nil {
		t.Error("Expected recent_deployments to be present")
	}
}
