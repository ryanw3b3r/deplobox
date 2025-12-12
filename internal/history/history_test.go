package history

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestHistory_RecordDeployment(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	hist, err := NewHistory(dbPath)
	if err != nil {
		t.Fatalf("Failed to create history: %v", err)
	}
	defer hist.Close()

	duration := 5.5
	commitHash := "abc123def456"
	record := &DeploymentRecord{
		Project:         "test-project",
		Branch:          "main",
		Ref:             "refs/heads/main",
		Status:          "success",
		DurationSeconds: &duration,
		CommitHash:      &commitHash,
	}

	id, err := hist.RecordDeployment(context.Background(), record)
	if err != nil {
		t.Fatalf("Failed to record deployment: %v", err)
	}

	if id == 0 {
		t.Error("Expected non-zero deployment ID")
	}
}

func TestHistory_GetLatestDeployment(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	hist, err := NewHistory(dbPath)
	if err != nil {
		t.Fatalf("Failed to create history: %v", err)
	}
	defer hist.Close()

	ctx := context.Background()

	// Record two deployments with different timestamps
	duration1 := 1.0
	_, err = hist.RecordDeployment(ctx, &DeploymentRecord{
		Project:         "test-project",
		Branch:          "main",
		Ref:             "refs/heads/main",
		Status:          "success",
		DurationSeconds: &duration1,
	})
	if err != nil {
		t.Fatalf("Failed to record first deployment: %v", err)
	}

	// Small delay to ensure different timestamp
	time.Sleep(10 * time.Millisecond)

	duration2 := 2.0
	_, err = hist.RecordDeployment(ctx, &DeploymentRecord{
		Project:         "test-project",
		Branch:          "main",
		Ref:             "refs/heads/main",
		Status:          "failed",
		DurationSeconds: &duration2,
	})
	if err != nil {
		t.Fatalf("Failed to record second deployment: %v", err)
	}

	// Get latest (should be the second one)
	latest, err := hist.GetLatestDeployment(ctx, "test-project")
	if err != nil {
		t.Fatalf("Failed to get latest deployment: %v", err)
	}

	if latest == nil {
		t.Fatal("Expected latest deployment to be non-nil")
	}

	if latest.Status != "failed" {
		t.Errorf("Expected latest status 'failed', got %q", latest.Status)
	}

	if latest.DurationSeconds == nil {
		t.Error("Expected duration to be non-nil")
	} else if *latest.DurationSeconds != 2.0 {
		t.Errorf("Expected duration 2.0, got %f", *latest.DurationSeconds)
	}
}

func TestHistory_GetLatestDeployment_NoRecords(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	hist, err := NewHistory(dbPath)
	if err != nil {
		t.Fatalf("Failed to create history: %v", err)
	}
	defer hist.Close()

	latest, err := hist.GetLatestDeployment(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("Expected no error for nonexistent project, got: %v", err)
	}

	if latest != nil {
		t.Errorf("Expected nil for nonexistent project, got: %v", latest)
	}
}

func TestHistory_GetDeploymentHistory(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	hist, err := NewHistory(dbPath)
	if err != nil {
		t.Fatalf("Failed to create history: %v", err)
	}
	defer hist.Close()

	ctx := context.Background()

	// Record 5 deployments with delays to ensure unique timestamps
	for i := 0; i < 5; i++ {
		duration := float64(i)
		_, err = hist.RecordDeployment(ctx, &DeploymentRecord{
			Project:         "test-project",
			Branch:          "main",
			Ref:             "refs/heads/main",
			Status:          "success",
			DurationSeconds: &duration,
		})
		if err != nil {
			t.Fatalf("Failed to record deployment %d: %v", i, err)
		}
		// Small delay to ensure different timestamps
		time.Sleep(5 * time.Millisecond)
	}

	// Get history with limit 3
	history, err := hist.GetDeploymentHistory(ctx, "test-project", 3)
	if err != nil {
		t.Fatalf("Failed to get deployment history: %v", err)
	}

	if len(history) != 3 {
		t.Errorf("Expected 3 records, got %d", len(history))
	}

	// Should be in descending order (most recent first)
	if history[0].DurationSeconds == nil {
		t.Error("Expected first record duration to be non-nil")
	} else if *history[0].DurationSeconds != 4.0 {
		t.Errorf("Expected first record duration 4.0, got %f", *history[0].DurationSeconds)
	}
}

func TestHistory_GetAllProjectsStatus(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	hist, err := NewHistory(dbPath)
	if err != nil {
		t.Fatalf("Failed to create history: %v", err)
	}
	defer hist.Close()

	ctx := context.Background()

	// Record deployments for two different projects
	duration1 := 1.0
	hist.RecordDeployment(ctx, &DeploymentRecord{
		Project:         "project1",
		Branch:          "main",
		Ref:             "refs/heads/main",
		Status:          "success",
		DurationSeconds: &duration1,
	})

	duration2 := 2.0
	hist.RecordDeployment(ctx, &DeploymentRecord{
		Project:         "project2",
		Branch:          "main",
		Ref:             "refs/heads/main",
		Status:          "failed",
		DurationSeconds: &duration2,
	})

	// Get all projects status
	status, err := hist.GetAllProjectsStatus(ctx)
	if err != nil {
		t.Fatalf("Failed to get all projects status: %v", err)
	}

	if len(status) != 2 {
		t.Errorf("Expected 2 projects, got %d", len(status))
	}

	if status["project1"] == nil {
		t.Error("Expected project1 to be present")
	}

	if status["project2"] == nil {
		t.Error("Expected project2 to be present")
	}

	if status["project1"].Status != "success" {
		t.Errorf("Expected project1 status 'success', got %q", status["project1"].Status)
	}

	if status["project2"].Status != "failed" {
		t.Errorf("Expected project2 status 'failed', got %q", status["project2"].Status)
	}
}
