package history

import "time"

// DeploymentRecord represents a single deployment event in the database
type DeploymentRecord struct {
	ID              int64
	Project         string
	Branch          string
	Ref             string
	Status          string // success, failed, skipped, rejected, in_progress
	StartedAt       time.Time
	CompletedAt     *time.Time // nullable
	DurationSeconds *float64   // nullable
	CommitHash      *string    // nullable
	ErrorMessage    *string    // nullable
}

// DeploymentStatus represents the latest status of a project
type DeploymentStatus struct {
	Project          string             `json:"project"`
	LatestDeployment *DeploymentRecord  `json:"latest_deployment,omitempty"`
	RecentHistory    []DeploymentRecord `json:"recent_history"`
}
