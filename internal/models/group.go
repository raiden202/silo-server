package models

import "time"

// Built-in group slugs. Built-in groups cannot be deleted; the
// administrators group cannot lose the admin permission or its last
// enabled member.
const (
	GroupSlugAdministrators = "administrators"
	GroupSlugUsers          = "users"
)

// Group represents a row in the groups table. Groups are the only source of
// authorization policy: a user's effective policy is the most-permissive
// union of their groups.
type Group struct {
	ID                       int
	Slug                     string
	Name                     string
	Description              string
	BuiltIn                  bool
	Permissions              []string
	LibraryIDs               []int // nil = all libraries
	MaxStreams               int
	MaxTranscodes            int
	MaxProfiles              int
	MaxPlaybackQuality       string // "" = unrestricted
	DownloadAllowed          bool
	DownloadTranscodeAllowed bool
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// CreateGroupInput contains the fields to create a new group. The slug is
// derived from Name. Nil pointer fields use the DB defaults.
type CreateGroupInput struct {
	Name                     string // required
	Description              string
	Permissions              []string
	LibraryIDs               []int // nil = all libraries
	MaxStreams               *int
	MaxTranscodes            *int
	MaxProfiles              *int
	MaxPlaybackQuality       *string
	DownloadAllowed          *bool
	DownloadTranscodeAllowed *bool
}

// UpdateGroupInput contains optional fields for updating a group.
// Nil means "don't update". LibraryIDs follows the same convention
// models.UpdateUserInput uses: a non-nil pointer to a nil slice means
// "all libraries"; a pointer to an empty slice means "none".
type UpdateGroupInput struct {
	Name                     *string
	Description              *string
	Permissions              *[]string
	LibraryIDs               *[]int // nil = don't update; *nil = all; *[] = none
	MaxStreams               *int
	MaxTranscodes            *int
	MaxProfiles              *int
	MaxPlaybackQuality       *string
	DownloadAllowed          *bool
	DownloadTranscodeAllowed *bool
}
