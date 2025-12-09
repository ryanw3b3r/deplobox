package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"deplobox/internal/history"
	"deplobox/internal/project"
	"deplobox/internal/server"
	"deplobox/pkg/fileutil"

	"github.com/spf13/cobra"
)

var (
	configFile string
	logFile    string
	dbPath     string
	host       string
	port       int
	testMode   bool
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the webhook server",
	Long: `Start the HTTP server to receive GitHub webhook requests.

The server will listen for push events and trigger deployments based on your project configuration.`,
	RunE: runServe,
}

func init() {
	// Flags for serve command
	serveCmd.Flags().StringVarP(&configFile, "config", "c", getEnvOrDefault("DEPLOBOX_CONFIG_FILE", ""), "Path to projects.yaml configuration file")
	serveCmd.Flags().StringVar(&logFile, "log", getEnvOrDefault("DEPLOBOX_LOG_FILE", "./deployments.log"), "Path to log file")
	serveCmd.Flags().StringVar(&dbPath, "db", getEnvOrDefault("DEPLOBOX_DB_PATH", "./deployments.db"), "Path to SQLite database")
	serveCmd.Flags().StringVar(&host, "host", getEnvOrDefault("DEPLOBOX_HOST", "127.0.0.1"), "Host to bind to")
	serveCmd.Flags().IntVarP(&port, "port", "p", getEnvOrDefaultInt("DEPLOBOX_PORT", 5000), "Port to listen on")
	serveCmd.Flags().BoolVar(&testMode, "test-mode", os.Getenv("DEPLOBOX_SKIP_VALIDATION") == "1", "Enable test mode (skip validation)")
}

func runServe(cmd *cobra.Command, args []string) error {
	// Determine config file path
	if configFile == "" {
		// Search in default locations using pkg/fileutil
		searchPaths := fileutil.DefaultConfigPaths("projects.yaml")
		configFile = fileutil.SearchPathsOptional(searchPaths)
		if configFile == "" {
			fmt.Fprintf(os.Stderr, "Error: No configuration file found in default locations:\n")
			for _, path := range searchPaths {
				fmt.Fprintf(os.Stderr, "  - %s\n", path)
			}
			fmt.Fprintf(os.Stderr, "Use --config flag to specify a custom location\n")
			return fmt.Errorf("configuration file not found")
		}
	}

	// Set up logging
	logger, logFileHandle, err := setupLogging(logFile)
	if err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}
	defer logFileHandle.Close()

	logger.Info("Starting deplobox")

	// Load configuration
	logger.Info("Loading configuration", "config", configFile)
	_, projects, err := project.LoadConfig(configFile)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err)
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	logger.Info("Configuration validated successfully", "count", len(projects))

	// Warn if no projects are configured
	if len(projects) == 0 {
		logger.Warn("No projects configured in config file", "config", configFile)
		logger.Warn("The server will start but won't handle any deployments until projects are added")
	}

	// Create project registry
	registry := project.NewRegistry(projects)

	// Initialize history database
	var hist *history.History
	if !testMode {
		logger.Info("Initializing history database", "db", dbPath)
		hist, err = history.NewHistory(dbPath)
		if err != nil {
			logger.Error("Failed to initialize history database", "error", err)
			return fmt.Errorf("failed to initialize history database: %w", err)
		}
		defer hist.Close()
	}

	// Create and start server
	srv := server.NewServer(registry, hist, logger, testMode)

	logger.Info("Starting HTTP server", "host", host, "port", port)
	if err := srv.Start(host, port); err != nil {
		logger.Error("Server failed", "error", err)
		return fmt.Errorf("server failed: %w", err)
	}

	return nil
}

// setupLogging configures slog for file logging
// Returns both the logger and the file handle (caller must close the file)
func setupLogging(logPath string) (*slog.Logger, *os.File, error) {
	// Create log directory if needed
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file with secure permissions
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Create multi-writer to log to both file and console
	multiWriter := io.MultiWriter(os.Stdout, file)

	// Create JSON handler for structured logging
	handler := slog.NewJSONHandler(multiWriter, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	logger := slog.New(handler)

	return logger, file, nil
}

// Helper functions for environment variables
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvOrDefaultInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intVal int
		if _, err := fmt.Sscanf(value, "%d", &intVal); err == nil {
			return intVal
		}
	}
	return defaultValue
}
