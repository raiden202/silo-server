package models

import "time"

// User represents a row in the users table plus the effective access policy
// derived from the user's group memberships at load time.
type User struct {
	ID                        int
	Email                     string
	Username                  string
	PasswordHash              string
	LocalPasswordLoginEnabled bool
	// IsAdmin, GroupIDs, and the policy fields below are derived from group
	// membership at load time (see auth.ApplyEffectivePolicy).
	IsAdmin                  bool
	GroupIDs                 []int
	Permissions              []string
	Enabled                  bool
	LibraryIDs               []int // effective; nil = all libraries
	MaxPlaybackQuality       string
	AccessPolicyRevision     int64
	MaxStreams               int
	MaxTranscodes            int
	MaxProfiles              int
	DownloadAllowed          bool
	DownloadTranscodeAllowed bool
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// CreateUserInput contains the fields required to create a new user.
// Access policy comes from group memberships, not per-user columns.
type CreateUserInput struct {
	Email                     string // required
	Username                  string // required
	Password                  string // plaintext, will be bcrypt-hashed
	LocalPasswordLoginEnabled *bool
	// GroupIDs are the group memberships to create. nil means the caller
	// resolves defaults via the AccountProvisioner.
	GroupIDs []int
}

// UpdateUserInput contains optional fields for updating a user.
// Pointer fields: nil means "don't update", non-nil means "set to this value".
type UpdateUserInput struct {
	Email                     *string
	Username                  *string
	Password                  *string // plaintext, will be bcrypt-hashed if provided
	LocalPasswordLoginEnabled *bool
	Enabled                   *bool
	// GroupIDs replaces the user's group memberships. nil = don't update.
	GroupIDs *[]int
}
