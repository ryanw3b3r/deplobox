package deployment

import (
	"context"
	"fmt"
	"log/slog"
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
	Logger       *slog.Logger
}

// NewDeployment creates a new deployment instance
func NewDeployment(proj *project.Project, payload map[string]interface{}, exposeOutput bool, logger *slog.Logger) *Deployment {
	return &Deployment{
		Project:      proj,
		Payload:      payload,
		ExposeOutput: exposeOutput,
		Outputs:      []string{},
		Executor:     NewExecutor(proj.Path),
		Logger:       logger,
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

// log logs a message if logger is available
func (d *Deployment) log(level slog.Level, msg string, args ...any) {
	if d.Logger != nil {
		d.Logger.Log(context.Background(), level, msg, args...)
	}
}

// logOutput logs command output (stdout/stderr) if non-empty
func (d *Deployment) logOutput(step string, result *ExecutionResult) {
	if d.Logger == nil || result == nil {
		return
	}
	if result.Stdout != "" {
		d.Logger.Info("command output", "project", d.Project.Name, "step", step, "stream", "stdout", "output", strings.TrimSpace(result.Stdout))
	}
	if result.Stderr != "" {
		d.Logger.Info("command output", "project", d.Project.Name, "step", step, "stream", "stderr", "output", strings.TrimSpace(result.Stderr))
	}
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
		d.log(slog.LevelWarn, "deployment cancelled before start", "project", d.Project.Name)
		return d.errorResponse("Deployment cancelled before start", nil), http.StatusRequestTimeout
	default:
	}

	// Check if we should deploy
	if !d.ShouldDeploy() {
		d.log(slog.LevelInfo, "skipping deployment - not target branch", "project", d.Project.Name, "branch", d.Project.Branch)
		return map[string]interface{}{
			"message": "Not target branch, skipping",
		}, http.StatusOK
	}

	// Validate branch name for security
	if err := security.ValidateBranchName(d.Project.Branch); err != nil {
		d.log(slog.LevelError, "invalid branch name", "project", d.Project.Name, "branch", d.Project.Branch, "error", err)
		return d.errorResponse(fmt.Sprintf("Invalid branch name: %v", err), nil), http.StatusBadRequest
	}

	// Validate project name for security
	if err := security.ValidateProjectName(d.Project.Name); err != nil {
		d.log(slog.LevelError, "invalid project name", "project", d.Project.Name, "error", err)
		return d.errorResponse(fmt.Sprintf("Invalid project name: %v", err), nil), http.StatusBadRequest
	}

	d.log(slog.LevelInfo, "starting deployment", "project", d.Project.Name, "branch", d.Project.Branch)

	// Step 1: Fresh clone into new release directory
	d.log(slog.LevelInfo, "step 1: cloning repository", "project", d.Project.Name, "branch", d.Project.Branch)
	releaseDir, createResult, err := d.Executor.CreateRelease(ctx, d.Project.Branch, d.Project.PullTimeout)
	if err != nil {
		if createResult != nil {
			d.Outputs = append(d.Outputs, createResult.Stdout, createResult.Stderr)
			d.logOutput("git_clone", createResult)
		}
		d.log(slog.LevelError, "failed to clone repository", "project", d.Project.Name, "error", err)
		return d.errorResponse(fmt.Sprintf("Failed to clone repository: %v", err), nil), http.StatusInternalServerError
	}
	d.Outputs = append(d.Outputs, createResult.Stdout, createResult.Stderr)
	d.logOutput("git_clone", createResult)
	d.log(slog.LevelInfo, "repository cloned", "project", d.Project.Name, "release_dir", releaseDir)

	// Check for cancellation before copying shared files
	select {
	case <-ctx.Done():
		d.log(slog.LevelWarn, "deployment cancelled during git clone", "project", d.Project.Name)
		return d.errorResponse("Deployment cancelled during git clone", nil), http.StatusRequestTimeout
	default:
	}

	// Step 2: Copy shared files to release
	d.log(slog.LevelInfo, "step 2: copying shared files", "project", d.Project.Name)
	sharedResult, err := d.Executor.CopySharedFiles(ctx, releaseDir, DefaultSharedFilesTimeout)
	if err != nil || !sharedResult.OK() {
		d.Outputs = append(d.Outputs, sharedResult.Stdout, sharedResult.Stderr)
		d.logOutput("copy_shared", sharedResult)
		errMsg := "Failed to copy shared files"
		if err != nil {
			errMsg = fmt.Sprintf("%s: %v", errMsg, err)
		}
		d.log(slog.LevelError, "failed to copy shared files", "project", d.Project.Name, "error", err)
		return d.errorResponse(errMsg, sharedResult), http.StatusInternalServerError
	}
	if sharedResult.Stdout != "" || sharedResult.Stderr != "" {
		d.Outputs = append(d.Outputs, sharedResult.Stdout, sharedResult.Stderr)
		d.logOutput("copy_shared", sharedResult)
	}
	d.log(slog.LevelInfo, "shared files copied", "project", d.Project.Name)

	// Step 3: Execute post-deploy commands if present
	if len(d.Project.PostDeploy) > 0 {
		d.log(slog.LevelInfo, "step 3: running post-deploy commands", "project", d.Project.Name, "command_count", len(d.Project.PostDeploy))
		postResults, err := d.Executor.RunPostDeployCommands(ctx, releaseDir, d.Project.PostDeploy, d.Project.PostDeployTimeout)

		// Collect and log all outputs
		for i, result := range postResults {
			d.Outputs = append(d.Outputs, result.Stdout, result.Stderr)
			d.logOutput(fmt.Sprintf("post_deploy[%d]", i), result)
		}

		if err != nil {
			d.log(slog.LevelError, "post-deploy command failed", "project", d.Project.Name, "error", err)
			return d.errorResponse(fmt.Sprintf("Post-deploy command failed: %v", err), nil), http.StatusInternalServerError
		}
		d.log(slog.LevelInfo, "post-deploy commands completed", "project", d.Project.Name, "commands_run", len(postResults))
	} else {
		d.log(slog.LevelInfo, "step 3: no post-deploy commands configured", "project", d.Project.Name)
	}

	// Step 4: Update current symlink (atomic cutover)
	d.log(slog.LevelInfo, "step 4: updating current symlink", "project", d.Project.Name, "release_dir", releaseDir)
	if err := d.Executor.UpdateCurrentSymlink(releaseDir); err != nil {
		d.log(slog.LevelError, "failed to update current symlink", "project", d.Project.Name, "error", err)
		return d.errorResponse(fmt.Sprintf("Failed to update current symlink: %v", err), nil), http.StatusInternalServerError
	}
	d.log(slog.LevelInfo, "current symlink updated", "project", d.Project.Name)

	// Step 5: Execute post-activate commands if present
	// These run after the deployment is activated (current symlink updated)
	if len(d.Project.PostActivate) > 0 {
		d.log(slog.LevelInfo, "step 5: running post-activate commands", "project", d.Project.Name, "command_count", len(d.Project.PostActivate))
		postActivateResults, err := d.Executor.RunPostActivateCommands(ctx, d.Project.PostActivate, d.Project.PostActivateTimeout)

		// Collect and log all outputs
		for i, result := range postActivateResults {
			d.Outputs = append(d.Outputs, result.Stdout, result.Stderr)
			d.logOutput(fmt.Sprintf("post_activate[%d]", i), result)
		}

		if err != nil {
			d.log(slog.LevelError, "post-activate command failed", "project", d.Project.Name, "error", err)
			return d.errorResponse(fmt.Sprintf("Post-activate command failed: %v", err), nil), http.StatusInternalServerError
		}
		d.log(slog.LevelInfo, "post-activate commands completed", "project", d.Project.Name, "commands_run", len(postActivateResults))
	} else {
		d.log(slog.LevelInfo, "step 5: no post-activate commands configured", "project", d.Project.Name)
	}

	// Step 6: Cleanup old releases
	d.log(slog.LevelInfo, "step 6: cleaning up old releases", "project", d.Project.Name, "keep_releases", DefaultKeepReleases)
	if err := d.Executor.CleanupOldReleases(DefaultKeepReleases); err != nil {
		// Log warning but don't fail
		d.log(slog.LevelWarn, "cleanup failed", "project", d.Project.Name, "error", err)
		d.Outputs = append(d.Outputs, fmt.Sprintf("Warning: cleanup failed: %v", err))
	} else {
		d.log(slog.LevelInfo, "cleanup completed", "project", d.Project.Name)
	}

	// Success
	d.log(slog.LevelInfo, "deployment completed successfully", "project", d.Project.Name)
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
