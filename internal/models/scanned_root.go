package models

import "time"

// ScannedMediaRoot is the persisted root-inference snapshot for one
// canonical root inside a library folder.
type ScannedMediaRoot struct {
	MediaFolderID     int
	RootPath          string
	State             string
	InferredType      string
	TypeConfidence    string
	Title             string
	Year              int
	TmdbID            string
	ImdbID            string
	TvdbID            string
	ObservedFileCount int
	SampleFilePath    string
	EvidenceJSON      []byte
	OverrideSource    string
	FirstSeenAt       time.Time
	LastSeenAt        time.Time
}

// MediaRootOverride stores an operator-provided override for a scanned root.
type MediaRootOverride struct {
	MediaFolderID   int
	RootPath        string
	ForcedType      string
	ForcedTitle     string
	ForcedYear      int
	ForcedTmdbID    string
	ForcedImdbID    string
	ForcedTvdbID    string
	Note            string
	CreatedByUserID *int
	UpdatedByUserID *int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
