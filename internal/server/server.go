package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"deplobox/internal/deployment"
	"deplobox/internal/history"
	"deplobox/internal/project"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

const (
	// HTTP server timeouts
	HTTPReadTimeout  = 10 * time.Second
	HTTPWriteTimeout = 10 * time.Second
	HTTPIdleTimeout  = 60 * time.Second

	// Request timeout for middleware
	RequestTimeout = 60 * time.Second

	// Rate limiting - requests per minute
	GlobalRateLimit  = 12 // Global rate limit per minute
	WebhookRateLimit = 4  // Webhook-specific rate limit per minute
)

// Server represents the HTTP server
type Server struct {
	Registry     *project.Registry
	History      *history.History
	LockManager  *deployment.LockManager
	Logger       *slog.Logger
	ExposeOutput bool
	TestMode     bool
	deployWg     sync.WaitGroup // Tracks in-flight async deployments
}

// NewServer creates a new server instance
func NewServer(registry *project.Registry, hist *history.History, logger *slog.Logger, testMode bool) *Server {
	exposeOutput := false
	exposeEnv := os.Getenv("DEPLOBOX_EXPOSE_OUTPUT")
	if exposeEnv == "1" || exposeEnv == "true" || exposeEnv == "yes" {
		exposeOutput = true
	}

	return &Server{
		Registry:     registry,
		History:      hist,
		LockManager:  deployment.NewLockManager(),
		Logger:       logger,
		ExposeOutput: exposeOutput,
		TestMode:     testMode,
	}
}

// Router creates and configures the HTTP router
func (s *Server) Router() *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(RequestTimeout))

	// Logging middleware
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				s.Logger.Info("http_request",
					"method", r.Method,
					"path", r.URL.Path,
					"status", ww.Status(),
					"duration_ms", time.Since(start).Milliseconds())
			}()

			next.ServeHTTP(ww, r)
		})
	})

	// Rate limiting middleware (only if not in test mode)
	if !s.TestMode {
		r.Use(NewRateLimitMiddleware(GlobalRateLimit, s.Logger))
	}

	// Routes
	r.Get("/health", s.HandleHealth)
	r.Get("/status/{projectName}", s.HandleStatus)

	// Webhook route with stricter rate limit
	if !s.TestMode {
		r.With(NewWebhookRateLimitMiddleware(WebhookRateLimit, s.Logger)).Post("/in/{projectName}", s.HandleWebhook)
	} else {
		r.Post("/in/{projectName}", s.HandleWebhook)
	}

	return r
}

// Start starts the HTTP server
func (s *Server) Start(host string, port int) error {
	addr := fmt.Sprintf("%s:%d", host, port)
	s.Logger.Info("Starting server", "addr", addr)

	server := &http.Server{
		Addr:         addr,
		Handler:      s.Router(),
		ReadTimeout:  HTTPReadTimeout,
		WriteTimeout: HTTPWriteTimeout,
		IdleTimeout:  HTTPIdleTimeout,
	}

	return server.ListenAndServe()
}

// WaitForDeployments waits for all in-flight async deployments to complete.
// This is primarily useful for testing.
func (s *Server) WaitForDeployments() {
	s.deployWg.Wait()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	// Wait for in-flight deployments
	s.deployWg.Wait()

	// Close history database connection
	if s.History != nil {
		return s.History.Close()
	}
	return nil
}
