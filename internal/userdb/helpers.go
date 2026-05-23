package userdb

import (
	"time"

	"github.com/google/uuid"
)

// generateUUID produces a version-4 UUID using the google/uuid package.
func generateUUID() string {
	return uuid.New().String()
}

// nowUTC returns the current time in ISO 8601 format (UTC).
func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}
