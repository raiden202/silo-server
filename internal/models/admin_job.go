package models

import (
	"encoding/json"
	"time"
)

// AdminJob represents a reusable background job row for admin workflows.
type AdminJob struct {
	ID                string
	JobType           string
	Status            string
	CreatedByUserID   int
	RequestPayload    json.RawMessage
	ResultPayload     json.RawMessage
	Message           string
	ErrorMessage      string
	ProgressCurrent   int
	ProgressTotal     int
	ArtifactBucket    string
	ArtifactKey       string
	ArtifactSizeBytes int64
	PublicURL         string
	RequestedAt       time.Time
	StartedAt         *time.Time
	CompletedAt       *time.Time
	HeartbeatAt       *time.Time
	ExpiresAt         *time.Time
	PublishedAt       *time.Time
	UpdatedAt         time.Time
}
