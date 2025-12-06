// Package server implements the HTTP server for the deplobox webhook receiver.
//
// This package provides:
//   - GitHub webhook endpoint handling with HMAC signature verification
//   - Per-IP rate limiting to prevent abuse and DDoS attacks
//   - Health and status endpoints for monitoring
//   - Structured logging of all HTTP requests
//
// The server integrates with other packages:
//   - internal/project: Project configuration and validation
//   - internal/deployment: Git pull and post-deploy command execution
//   - internal/history: SQLite-based deployment history tracking
//
// Security features:
//   - HMAC-SHA256 webhook signature verification
//   - Content-Type validation (application/json only)
//   - Payload size limits (1MB max)
//   - Rate limiting (global and per-webhook)
//   - Per-project deployment locking (prevents concurrent deployments)
package server
