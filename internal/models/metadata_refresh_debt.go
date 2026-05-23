package models

import "time"

// MetadataRefreshDebt represents a durable scheduled metadata follow-up row.
type MetadataRefreshDebt struct {
	TargetType     string
	ContentID      string
	Priority       int
	ReasonMask     int64
	NextRefreshAt  time.Time
	ClaimedAt      *time.Time
	LeaseExpiresAt *time.Time
	LastAttemptAt  *time.Time
	LastSuccessAt  *time.Time
	AttemptCount   int
	LastError      string
	UpdatedAt      time.Time
}
