package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"deplobox/internal/history"
	"deplobox/internal/project"
	"deplobox/internal/server"

	"github.com/rs/zerolog"
)

func main() {
	// Parse command-line flags
	configFile := flag.String("config", getEnvOrDefault("DEPLOBOX_CONFIG_FILE", "./projects.yaml"), "Path to configuration file")
	logFile := flag.String("log", getEnvOrDefault("DEPLOBOX_LOG_FILE", "./deployments.log"), "Path to log file")
	dbPath := flag.String("db", getEnvOrDefault("DEPLOBOX_DB_PATH", "./deployments.db"), "Path to SQLite database")
	host := flag.String("host", getEnvOrDefault("DEPLOBOX_HOST", "127.0.0.1"), "Host to bind to")
	port := flag.Int("port", getEnvOrDefaultInt("DEPLOBOX_PORT", 5000), "Port to listen on")
	testMode := flag.Bool("test-mode", os.Getenv("DEPLOBOX_SKIP_VALIDATION") == "1", "Enable test mode (skip validation)")

	flag.Parse()

	// Set up logging
	logger, logFileHandle := setupLogging(*logFile)
	defer logFileHandle.Close()
	logger.Info().Msg("Starting deplobox")

	// Load configuration
	logger.Info().Str("config", *configFile).Msg("Loading configuration")
	_, projects, err := project.LoadConfig(*configFile)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load configuration")
	}

	logger.Info().Int("count", len(projects)).Msg("Configuration validated successfully")

	// Create project registry
	registry := project.NewRegistry(projects)

	// Initialize history database
	var hist *history.History
	if !*testMode {
		logger.Info().Str("db", *dbPath).Msg("Initializing history database")
		hist, err = history.NewHistory(*dbPath)
		if err != nil {
			logger.Fatal().Err(err).Msg("Failed to initialize history database")
		}
		defer hist.Close()
	}

	// Create and start server
	srv := server.NewServer(registry, hist, &logger, *testMode)

	logger.Info().Str("host", *host).Int("port", *port).Msg("Starting HTTP server")
	if err := srv.Start(*host, *port); err != nil {
		logger.Fatal().Err(err).Msg("Server failed")
	}
}

// setupLogging configures zerolog for file logging
// Returns both the logger and the file handle (caller must close the file)
func setupLogging(logFile string) (zerolog.Logger, *os.File) {
	// Create log directory if needed
	logDir := filepath.Dir(logFile)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log directory: %v\n", err)
		os.Exit(1)
	}

	// Open log file
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open log file: %v\n", err)
		os.Exit(1)
	}

	// Create multi-writer to log to both file and console
	multiWriter := io.MultiWriter(os.Stdout, file)

	// Create logger with both outputs
	logger := zerolog.New(multiWriter).With().Timestamp().Logger()

	return logger, file
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
