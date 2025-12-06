package deployment

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"deplobox/internal/project"
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

// Execute runs the full deployment process
func (d *Deployment) Execute(ctx context.Context) (map[string]interface{}, int) {
	// Check if we should deploy
	if !d.ShouldDeploy() {
		return map[string]interface{}{
			"message": "Not target branch, skipping",
		}, http.StatusOK
	}

	// Execute git pull
	pullResult, err := d.Executor.RunGitPull(ctx, d.Project.Branch, d.Project.PullTimeout)
	if err != nil || !pullResult.OK() {
		d.Outputs = append(d.Outputs, pullResult.Stdout, pullResult.Stderr)
		errMsg := "Git pull failed"
		if err != nil {
			errMsg = fmt.Sprintf("%s: %v", errMsg, err)
		}
		return d.errorResponse(errMsg, pullResult), http.StatusInternalServerError
	}

	d.Outputs = append(d.Outputs, pullResult.Stdout, pullResult.Stderr)

	// Execute post-deploy commands if present
	if len(d.Project.PostDeploy) > 0 {
		postResults, err := d.Executor.RunPostDeployCommands(ctx, d.Project.PostDeploy, d.Project.PostDeployTimeout)

		// Collect all outputs
		for _, result := range postResults {
			d.Outputs = append(d.Outputs, result.Stdout, result.Stderr)
		}

		if err != nil {
			return d.errorResponse(fmt.Sprintf("Post-deploy command failed: %v", err), nil), http.StatusInternalServerError
		}
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
