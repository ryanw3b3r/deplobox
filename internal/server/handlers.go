package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"deplobox/internal/deployment"
	"deplobox/internal/history"
	"deplobox/internal/project"
	"deplobox/internal/security"

	"github.com/go-chi/chi/v5"
)

const (
	MaxPayloadBytes     = 1_000_000 // 1 MB
	RecentDeploymentsLimit = 10        // Number of recent deployments to return in status endpoint
)

// HandleWebhook handles GitHub webhook requests
func (s *Server) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	projectName := chi.URLParam(r, "projectName")
	s.Logger.Info("webhook received", "project", projectName, "remote_addr", r.RemoteAddr)

	// Validate project name for security
	if err := security.ValidateProjectName(projectName); err != nil {
		s.Logger.Warn("Invalid project name in webhook request", "project", projectName, "error", err)
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Invalid project name: %v", err)})
		return
	}

	// Check if project exists
	s.Logger.Debug("looking up project", "project", projectName)
	proj, err := s.Registry.Get(projectName)
	if err != nil {
		s.Logger.Warn("project not found", "project", projectName)
		s.respondJSON(w, http.StatusNotFound, map[string]string{"error": "Unknown project"})
		return
	}
	s.Logger.Debug("project found", "project", projectName, "branch", proj.Branch)

	// Check payload size (ContentLength can be -1 if not set, so check for both > 0 and > max)
	if r.ContentLength > MaxPayloadBytes {
		s.Logger.Warn("payload too large", "project", projectName, "size", r.ContentLength)
		s.respondJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "Payload too large"})
		return
	}

	contentType := r.Header.Get("Content-Type")
	s.Logger.Debug("content type check", "project", projectName, "content_type", contentType)
	if contentType != "application/json" {
		s.Logger.Warn("invalid content type", "project", projectName, "content_type", contentType)
		s.respondJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "Invalid content type"})
		return
	}

	// Read payload
	body, err := io.ReadAll(io.LimitReader(r.Body, MaxPayloadBytes))
	if err != nil {
		s.Logger.Error("failed to read request body", "error", err, "project", projectName)
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to read payload"})
		return
	}
	s.Logger.Debug("payload read", "project", projectName, "bytes", len(body))

	// Verify signature
	signature := r.Header.Get("X-Hub-Signature-256")
	s.Logger.Debug("verifying signature", "project", projectName, "signature_present", signature != "")
	if !VerifySignature(body, signature, proj.Secret) {
		s.Logger.Warn("invalid signature", "project", projectName)
		s.respondJSON(w, http.StatusForbidden, map[string]string{"error": "Invalid signature"})
		return
	}
	s.Logger.Debug("signature verified", "project", projectName)

	// Parse JSON payload
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		s.Logger.Error("failed to parse JSON payload", "error", err, "project", projectName)
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON payload"})
		return
	}

	if len(payload) == 0 {
		s.Logger.Info("empty payload, skipping", "project", projectName)
		s.respondJSON(w, http.StatusOK, map[string]string{"message": "Missing payload, skipping"})
		return
	}

	// Extract ref for logging
	ref, _ := payload["ref"].(string)
	s.Logger.Info("payload parsed", "project", projectName, "ref", ref, "target_branch", proj.Branch)

	// Check if this is a target branch before acquiring lock
	// This allows us to respond immediately for non-target branches
	deploy := deployment.NewDeployment(proj, payload, s.ExposeOutput, s.Logger)
	shouldDeploy := deploy.ShouldDeploy()
	s.Logger.Debug("checking if should deploy", "project", projectName, "ref", ref, "target_branch", proj.Branch, "should_deploy", shouldDeploy)
	if !shouldDeploy {
		s.Logger.Info("not target branch, skipping", "project", projectName, "ref", ref, "target_branch", proj.Branch)
		s.respondJSON(w, http.StatusOK, map[string]string{"message": "Not target branch, skipping"})
		return
	}

	// Try to acquire deployment lock
	s.Logger.Debug("attempting to acquire lock", "project", projectName)
	if !s.LockManager.TryLock(projectName) {
		s.Logger.Warn("deployment already in progress, rejecting", "project", projectName)

		// Record rejected deployment
		if !s.TestMode {
			ref, _ := payload["ref"].(string)
			if _, err := s.History.RecordDeployment(r.Context(), &history.DeploymentRecord{
				Project:      projectName,
				Branch:       proj.Branch,
				Ref:          ref,
				Status:       "rejected",
				ErrorMessage: stringPtr("Deployment already in progress"),
			}); err != nil {
				s.Logger.Error("Failed to record rejection in history", "error", err, "project", projectName)
			}
		}

		s.respondJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Deployment already in progress"})
		return
	}

	s.Logger.Info("lock acquired, starting async deployment", "project", projectName)

	// Respond immediately to GitHub to avoid timeout
	// GitHub webhooks have a 10-second timeout, so we acknowledge receipt
	// and process the deployment asynchronously
	s.respondJSON(w, http.StatusAccepted, map[string]string{
		"message": "Deployment accepted",
		"project": projectName,
	})

	// Execute deployment asynchronously
	s.deployWg.Add(1)
	s.Logger.Info("spawning deployment goroutine", "project", projectName)
	go func() {
		defer s.deployWg.Done()
		defer s.LockManager.Unlock(projectName)
		s.Logger.Info("deployment goroutine started", "project", projectName)
		s.executeDeployment(context.Background(), projectName, proj, payload)
	}()
}

// executeDeployment runs the deployment and records history
func (s *Server) executeDeployment(ctx context.Context, projectName string, proj *project.Project, payload map[string]interface{}) {
	s.Logger.Info("executeDeployment: starting", "project", projectName)
	startTime := time.Now()

	// Create deployment
	deploy := deployment.NewDeployment(proj, payload, s.ExposeOutput, s.Logger)

	// Execute
	response, statusCode := deploy.Execute(ctx)

	// Calculate duration
	duration := time.Since(startTime).Seconds()

	// Record history
	if !s.TestMode {
		ref, _ := payload["ref"].(string)
		commitHash, _ := payload["after"].(string)

		var status string
		var errorMsg *string

		if statusCode == 200 {
			if msg, ok := response["message"].(string); ok && msg == "Deployment successful" {
				status = "success"
			} else {
				status = "skipped"
			}
		} else {
			status = "failed"
			if errStr, ok := response["error"].(string); ok {
				errorMsg = &errStr
			}
		}

		_, err := s.History.RecordDeployment(ctx, &history.DeploymentRecord{
			Project:         projectName,
			Branch:          proj.Branch,
			Ref:             ref,
			Status:          status,
			DurationSeconds: &duration,
			CommitHash:      stringPtrOrNil(commitHash),
			ErrorMessage:    errorMsg,
		})

		if err != nil {
			s.Logger.Error("Failed to record deployment history", "error", err, "project", projectName)
		}
	}

	// Log final status (we already responded to GitHub)
	if statusCode == 200 {
		s.Logger.Info("deployment completed", "project", projectName, "status", "success")
	} else {
		s.Logger.Error("deployment failed", "project", projectName, "response", response)
	}
}

// HandleHealth handles health check requests
func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	projectNames := s.Registry.List()

	response := map[string]interface{}{
		"status":        "ok",
		"projects":      projectNames,
		"project_count": s.Registry.Count(),
	}

	s.respondJSON(w, http.StatusOK, response)
}

// HandleStatus handles deployment status requests
func (s *Server) HandleStatus(w http.ResponseWriter, r *http.Request) {
	projectName := chi.URLParam(r, "projectName")

	// Validate project name for security
	if err := security.ValidateProjectName(projectName); err != nil {
		s.Logger.Warn("Invalid project name in status request", "project", projectName, "error", err)
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Invalid project name: %v", err)})
		return
	}

	// Check if project exists
	_, err := s.Registry.Get(projectName)
	if err != nil {
		s.respondJSON(w, http.StatusNotFound, map[string]string{"error": "Unknown project"})
		return
	}

	// Check if history is available
	if s.TestMode {
		s.respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "History not available in test mode"})
		return
	}

	// Get latest deployment
	latest, err := s.History.GetLatestDeployment(r.Context(), projectName)
	if err != nil {
		s.Logger.Error("Failed to get latest deployment", "error", err, "project", projectName)
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch deployment status"})
		return
	}

	// Get recent deployments
	recent, err := s.History.GetDeploymentHistory(r.Context(), projectName, RecentDeploymentsLimit)
	if err != nil {
		s.Logger.Error("Failed to get deployment history", "error", err, "project", projectName)
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch deployment status"})
		return
	}

	response := map[string]interface{}{
		"project":            projectName,
		"latest_deployment":  latest,
		"recent_deployments": recent,
	}

	s.respondJSON(w, http.StatusOK, response)
}

// respondJSON sends a JSON response
func (s *Server) respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.Logger.Error("failed to encode JSON response", "error", err, "status", statusCode)
	} else {
		s.Logger.Debug("json response sent", "status", statusCode)
	}
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func stringPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
