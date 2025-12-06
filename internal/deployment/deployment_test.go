package deployment

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"deplobox/internal/project"
)

func TestDeployment_ShouldDeploy(t *testing.T) {
	testProject := &project.Project{
		Name:   "test",
		Branch: "main",
	}

	testCases := []struct {
		name     string
		payload  map[string]interface{}
		expected bool
	}{
		{
			name:     "matching branch",
			payload:  map[string]interface{}{"ref": "refs/heads/main"},
			expected: true,
		},
		{
			name:     "non-matching branch",
			payload:  map[string]interface{}{"ref": "refs/heads/develop"},
			expected: false,
		},
		{
			name:     "tag ref",
			payload:  map[string]interface{}{"ref": "refs/tags/v1.0"},
			expected: false,
		},
		{
			name:     "missing ref",
			payload:  map[string]interface{}{},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			deploy := NewDeployment(testProject, tc.payload, false)
			result := deploy.ShouldDeploy()

			if result != tc.expected {
				t.Errorf("ShouldDeploy() = %v, expected %v for payload %v",
					result, tc.expected, tc.payload)
			}
		})
	}
}

func TestDeployment_Execute_SkipNonTargetBranch(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	os.Mkdir(gitDir, 0755)

	testProject := &project.Project{
		Name:   "test",
		Path:   tmpDir,
		Branch: "main",
	}

	payload := map[string]interface{}{
		"ref": "refs/heads/develop", // Not main
	}

	deploy := NewDeployment(testProject, payload, false)
	response, statusCode := deploy.Execute(context.Background())

	if statusCode != 200 {
		t.Errorf("Expected status 200, got %d", statusCode)
	}

	if response["message"] != "Not target branch, skipping" {
		t.Errorf("Expected skip message, got %v", response)
	}
}

func TestDeployment_Execute_OutputVisibility(t *testing.T) {
	tmpDir := t.TempDir()
	gitDir := filepath.Join(tmpDir, ".git")
	os.Mkdir(gitDir, 0755)

	testProject := &project.Project{
		Name:       "test",
		Path:       tmpDir,
		Branch:     "develop",
		PostDeploy: []interface{}{"echo test"},
	}

	payload := map[string]interface{}{
		"ref": "refs/heads/develop",
	}

	// Test with output hidden (default)
	deployHidden := NewDeployment(testProject, payload, false)
	responseHidden, _ := deployHidden.Execute(context.Background())

	if _, hasOutput := responseHidden["output"]; hasOutput {
		t.Error("Expected output to be hidden when ExposeOutput=false")
	}

	// Test with output exposed
	deployExposed := NewDeployment(testProject, payload, true)
	responseExposed, _ := deployExposed.Execute(context.Background())

	if _, hasOutput := responseExposed["output"]; !hasOutput {
		t.Error("Expected output to be present when ExposeOutput=true")
	}
}
