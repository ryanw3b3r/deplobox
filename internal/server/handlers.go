package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"deplobox/internal/deployment"
	"deplobox/internal/history"
	"deplobox/internal/project"

	"github.com/go-chi/chi/v5"
)

const (
	MaxPayloadBytes     = 1_000_000 // 1 MB
	RecentDeploymentsLimit = 10        // Number of recent deployments to return in status endpoint
)

// HandleWebhook handles GitHub webhook requests
func (s *Server) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	projectName := chi.URLParam(r, "projectName")

	// Check if project exists
	proj, err := s.Registry.Get(projectName)
	if err != nil {
		s.respondJSON(w, http.StatusNotFound, map[string]string{"error": "Unknown project"})
		return
	}

	// Check payload size (ContentLength can be -1 if not set, so check for both > 0 and > max)
	if r.ContentLength > MaxPayloadBytes {
		s.respondJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "Payload too large"})
		return
	}

	// Check content type
	if r.Header.Get("Content-Type") != "application/json" {
		s.respondJSON(w, http.StatusUnsupportedMediaType, map[string]string{"error": "Invalid content type"})
		return
	}

	// Check event type
	if r.Header.Get("X-GitHub-Event") != "push" {
		s.respondJSON(w, http.StatusOK, map[string]string{"message": "Ignoring non-push event"})
		return
	}

	// Read payload
	body, err := io.ReadAll(io.LimitReader(r.Body, MaxPayloadBytes))
	if err != nil {
		s.Logger.Error().Err(err).Str("project", projectName).Msg("Failed to read request body")
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to read payload"})
		return
	}

	// Verify signature
	signature := r.Header.Get("X-Hub-Signature-256")
	if !VerifySignature(body, signature, proj.Secret) {
		s.respondJSON(w, http.StatusForbidden, map[string]string{"error": "Invalid signature"})
		return
	}

	// Parse JSON payload
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		s.Logger.Error().Err(err).Str("project", projectName).Msg("Failed to parse JSON payload")
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON payload"})
		return
	}

	if len(payload) == 0 {
		s.respondJSON(w, http.StatusOK, map[string]string{"message": "Missing payload, skipping"})
		return
	}

	// Try to acquire deployment lock
	if !s.LockManager.TryLock(projectName) {
		s.Logger.Warn().Str("project", projectName).Msg("Deployment already in progress, rejecting")

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
				s.Logger.Error().Err(err).Str("project", projectName).Msg("Failed to record rejection in history")
			}
		}

		s.respondJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Deployment already in progress"})
		return
	}

	defer s.LockManager.Unlock(projectName)

	// Execute deployment
	s.executeDeployment(r.Context(), w, projectName, proj, payload)
}

// executeDeployment runs the deployment and records history
func (s *Server) executeDeployment(ctx context.Context, w http.ResponseWriter, projectName string, proj *project.Project, payload map[string]interface{}) {
	startTime := time.Now()

	// Create deployment
	deploy := deployment.NewDeployment(proj, payload, s.ExposeOutput)

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
			s.Logger.Error().Err(err).Str("project", projectName).Msg("Failed to record deployment history")
		}
	}

	s.respondJSON(w, statusCode, response)
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
		s.Logger.Error().Err(err).Str("project", projectName).Msg("Failed to get latest deployment")
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch deployment status"})
		return
	}

	// Get recent deployments
	recent, err := s.History.GetDeploymentHistory(r.Context(), projectName, RecentDeploymentsLimit)
	if err != nil {
		s.Logger.Error().Err(err).Str("project", projectName).Msg("Failed to get deployment history")
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
		s.Logger.Error().Err(err).Msg("Failed to encode JSON response")
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
