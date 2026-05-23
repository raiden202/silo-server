package models

import "time"

// StaleMediaID records an external provider ID that returned 404 during refresh.
type StaleMediaID struct {
	ContentID   string
	Provider    string
	ProviderID  string
	FirstSeenAt time.Time
	LastSeenAt  time.Time
}
