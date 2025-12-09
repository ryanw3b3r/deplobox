package deployment

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"deplobox/internal/project"
	"deplobox/internal/security"
)

const (
	// DefaultSharedFilesTimeout is the default timeout for copying shared files
	DefaultSharedFilesTimeout = 30

	// DefaultKeepReleases is the number of releases to keep after cleanup
	DefaultKeepReleases = 5
)

// Deployment manages the execution of a deployment for a project
type Deployment struct {
	Project      *project.Project
	Payload      map[string]interface{}
	ExposeOutput bool
	Outputs      []string
	Executor     *Executor
}

// NewDeployment creates a new deployment instance
func NewDeployment(proj *project.Project, payload map[string]interface{}, exposeOutput bool) *Deployment {
	return &Deployment{
		Project:      proj,
		Payload:      payload,
		ExposeOutput: exposeOutput,
		Outputs:      []string{},
		Executor:     NewExecutor(proj.Path),
	}
}

// ShouldDeploy checks if deployment should proceed based on payload
func (d *Deployment) ShouldDeploy() bool {
	ref, ok := d.Payload["ref"].(string)
	if !ok {
		return false
	}
	return d.Project.MatchesRef(ref)
}

// Execute runs the full zero-downtime deployment process
func (d *Deployment) Execute(ctx context.Context) (map[string]interface{}, int) {
	// Validate context
	if ctx == nil {
		ctx = context.Background()
	}

	// Check for cancellation before starting
	select {
	case <-ctx.Done():
		return d.errorResponse("Deployment cancelled before start", nil), http.StatusRequestTimeout
	default:
	}

	// Check if we should deploy
	if !d.ShouldDeploy() {
		return map[string]interface{}{
			"message": "Not target branch, skipping",
		}, http.StatusOK
	}

	// Validate branch name for security
	if err := security.ValidateBranchName(d.Project.Branch); err != nil {
		return d.errorResponse(fmt.Sprintf("Invalid branch name: %v", err), nil), http.StatusBadRequest
	}

	// Validate project name for security
	if err := security.ValidateProjectName(d.Project.Name); err != nil {
		return d.errorResponse(fmt.Sprintf("Invalid project name: %v", err), nil), http.StatusBadRequest
	}

	// Step 1: Create new release directory
	releaseDir, createResult, err := d.Executor.CreateRelease(ctx, d.Project.Branch, d.Project.PullTimeout)
	if err != nil {
		if createResult != nil {
			d.Outputs = append(d.Outputs, createResult.Stdout, createResult.Stderr)
		}
		return d.errorResponse(fmt.Sprintf("Failed to create release: %v", err), nil), http.StatusInternalServerError
	}
	d.Outputs = append(d.Outputs, createResult.Stdout, createResult.Stderr)

	// Step 2: Pull latest changes from git
	pullResult, err := d.Executor.RunGitPull(ctx, releaseDir, d.Project.Branch, d.Project.PullTimeout)
	if err != nil || !pullResult.OK() {
		d.Outputs = append(d.Outputs, pullResult.Stdout, pullResult.Stderr)
		errMsg := "Git pull failed"
		if err != nil {
			errMsg = fmt.Sprintf("%s: %v", errMsg, err)
		}
		return d.errorResponse(errMsg, pullResult), http.StatusInternalServerError
	}
	d.Outputs = append(d.Outputs, pullResult.Stdout, pullResult.Stderr)

	// Check for cancellation before copying shared files
	select {
	case <-ctx.Done():
		return d.errorResponse("Deployment cancelled during git pull", nil), http.StatusRequestTimeout
	default:
	}

	// Step 3: Copy shared files to release
	sharedResult, err := d.Executor.CopySharedFiles(ctx, releaseDir, DefaultSharedFilesTimeout)
	if err != nil || !sharedResult.OK() {
		d.Outputs = append(d.Outputs, sharedResult.Stdout, sharedResult.Stderr)
		errMsg := "Failed to copy shared files"
		if err != nil {
			errMsg = fmt.Sprintf("%s: %v", errMsg, err)
		}
		return d.errorResponse(errMsg, sharedResult), http.StatusInternalServerError
	}
	if sharedResult.Stdout != "" || sharedResult.Stderr != "" {
		d.Outputs = append(d.Outputs, sharedResult.Stdout, sharedResult.Stderr)
	}

	// Step 4: Execute post-deploy commands if present
	if len(d.Project.PostDeploy) > 0 {
		postResults, err := d.Executor.RunPostDeployCommands(ctx, releaseDir, d.Project.PostDeploy, d.Project.PostDeployTimeout)

		// Collect all outputs
		for _, result := range postResults {
			d.Outputs = append(d.Outputs, result.Stdout, result.Stderr)
		}

		if err != nil {
			return d.errorResponse(fmt.Sprintf("Post-deploy command failed: %v", err), nil), http.StatusInternalServerError
		}
	}

	// Step 5: Update current symlink (atomic cutover)
	if err := d.Executor.UpdateCurrentSymlink(releaseDir); err != nil {
		return d.errorResponse(fmt.Sprintf("Failed to update current symlink: %v", err), nil), http.StatusInternalServerError
	}

	// Step 6: Execute post-activate commands if present
	// These run after the deployment is activated (current symlink updated)
	if len(d.Project.PostActivate) > 0 {
		postActivateResults, err := d.Executor.RunPostActivateCommands(ctx, d.Project.PostActivate, d.Project.PostActivateTimeout)

		// Collect all outputs
		for _, result := range postActivateResults {
			d.Outputs = append(d.Outputs, result.Stdout, result.Stderr)
		}

		if err != nil {
			return d.errorResponse(fmt.Sprintf("Post-activate command failed: %v", err), nil), http.StatusInternalServerError
		}
	}

	// Step 7: Cleanup old releases
	// Don't fail the deployment if cleanup fails
	if err := d.Executor.CleanupOldReleases(DefaultKeepReleases); err != nil {
		// Log warning but don't fail
		d.Outputs = append(d.Outputs, fmt.Sprintf("Warning: cleanup failed: %v", err))
	}

	// Success
	return d.successResponse(), http.StatusOK
}

// errorResponse builds an error response
func (d *Deployment) errorResponse(errorMsg string, result *ExecutionResult) map[string]interface{} {
	response := map[string]interface{}{
		"error": errorMsg,
	}

	if d.ExposeOutput {
		response["output"] = strings.Join(d.Outputs, "\n")
	}

	return response
}

// successResponse builds a success response
func (d *Deployment) successResponse() map[string]interface{} {
	response := map[string]interface{}{
		"message": "Deployment successful",
	}

	if d.ExposeOutput {
		response["output"] = strings.Join(d.Outputs, "\n")
	}

	return response
}
