package history

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// History manages deployment history in SQLite
type History struct {
	db *sql.DB
}

// NewHistory creates a new history tracker
func NewHistory(dbPath string) (*History, error) {
	// Open database connection
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool for SQLite (single writer)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	h := &History{db: db}

	// Initialize schema
	if err := h.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return h, nil
}

// Close closes the database connection
func (h *History) Close() error {
	return h.db.Close()
}

// initSchema creates the database tables and indexes
func (h *History) initSchema() error {
	_, err := h.db.Exec(`
		CREATE TABLE IF NOT EXISTS deployments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			project TEXT NOT NULL,
			branch TEXT NOT NULL,
			ref TEXT NOT NULL,
			status TEXT NOT NULL,
			started_at TEXT NOT NULL,
			completed_at TEXT,
			duration_seconds REAL,
			commit_hash TEXT,
			error_message TEXT
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// Create index for efficient queries
	_, err = h.db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_project_started
		ON deployments(project, started_at DESC)
	`)
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	return nil
}

// RecordDeployment records a deployment in the history
func (h *History) RecordDeployment(ctx context.Context, record *DeploymentRecord) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	var completedAt *string
	if record.CompletedAt != nil {
		formatted := record.CompletedAt.UTC().Format(time.RFC3339)
		completedAt = &formatted
	} else if record.Status != "in_progress" {
		completedAt = &now
	}

	result, err := h.db.ExecContext(ctx, `
		INSERT INTO deployments
		(project, branch, ref, status, started_at, completed_at,
		 duration_seconds, commit_hash, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		record.Project,
		record.Branch,
		record.Ref,
		record.Status,
		now,
		completedAt,
		record.DurationSeconds,
		record.CommitHash,
		record.ErrorMessage,
	)

	if err != nil {
		return 0, fmt.Errorf("failed to insert deployment record: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	return id, nil
}

// GetLatestDeployment returns the most recent deployment for a project
func (h *History) GetLatestDeployment(ctx context.Context, project string) (*DeploymentRecord, error) {
	row := h.db.QueryRowContext(ctx, `
		SELECT id, project, branch, ref, status, started_at, completed_at,
		       duration_seconds, commit_hash, error_message
		FROM deployments
		WHERE project = ?
		ORDER BY id DESC
		LIMIT 1
	`, project)

	record, err := scanDeploymentRecord(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query latest deployment: %w", err)
	}

	return record, nil
}

// GetDeploymentHistory returns deployment history for a project
func (h *History) GetDeploymentHistory(ctx context.Context, project string, limit int) ([]DeploymentRecord, error) {
	rows, err := h.db.QueryContext(ctx, `
		SELECT id, project, branch, ref, status, started_at, completed_at,
		       duration_seconds, commit_hash, error_message
		FROM deployments
		WHERE project = ?
		ORDER BY id DESC
		LIMIT ?
	`, project, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query deployment history: %w", err)
	}
	defer rows.Close()

	var records []DeploymentRecord
	for rows.Next() {
		record, err := scanDeploymentRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan deployment record: %w", err)
		}
		records = append(records, *record)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return records, nil
}

// GetAllProjectsStatus returns the latest deployment for each project
func (h *History) GetAllProjectsStatus(ctx context.Context) (map[string]*DeploymentRecord, error) {
	rows, err := h.db.QueryContext(ctx, `
		SELECT d1.id, d1.project, d1.branch, d1.ref, d1.status, d1.started_at,
		       d1.completed_at, d1.duration_seconds, d1.commit_hash, d1.error_message
		FROM deployments d1
		INNER JOIN (
			SELECT project, MAX(started_at) as max_started
			FROM deployments
			GROUP BY project
		) d2
		ON d1.project = d2.project AND d1.started_at = d2.max_started
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query all projects status: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*DeploymentRecord)
	for rows.Next() {
		record, err := scanDeploymentRecord(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan deployment record: %w", err)
		}
		result[record.Project] = record
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return result, nil
}

// scanner is an interface that both *sql.Row and *sql.Rows implement
type scanner interface {
	Scan(dest ...interface{}) error
}

// scanDeploymentRecord scans a database row into a DeploymentRecord
// Works with both *sql.Row and *sql.Rows
func scanDeploymentRecord(s scanner) (*DeploymentRecord, error) {
	var record DeploymentRecord
	var startedAtStr string
	var completedAtStr sql.NullString

	err := s.Scan(
		&record.ID,
		&record.Project,
		&record.Branch,
		&record.Ref,
		&record.Status,
		&startedAtStr,
		&completedAtStr,
		&record.DurationSeconds,
		&record.CommitHash,
		&record.ErrorMessage,
	)

	if err != nil {
		return nil, err
	}

	// Parse timestamps
	startedAt, err := time.Parse(time.RFC3339, startedAtStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse started_at timestamp: %w", err)
	}
	record.StartedAt = startedAt

	if completedAtStr.Valid {
		completedAt, err := time.Parse(time.RFC3339, completedAtStr.String)
		if err != nil {
			return nil, fmt.Errorf("failed to parse completed_at timestamp: %w", err)
		}
		record.CompletedAt = &completedAt
	}

	return &record, nil
}
