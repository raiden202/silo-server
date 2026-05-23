package models

import "time"

// SkippedMediaRoot represents a media root that was skipped during scanning.
type SkippedMediaRoot struct {
	MediaFolderID  int
	RootPath       string
	Reason         string
	SampleFilePath string
	FileCount      int
	FirstSeenAt    time.Time
	LastSeenAt     time.Time
}
