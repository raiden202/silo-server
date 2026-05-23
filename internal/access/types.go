package access

// Scope is the resolved effective access policy for a viewer request.
type Scope struct {
	UserID              int
	ProfileID           string
	AllowedLibraryIDs   []int
	DisabledLibraryIDs  []int // user-disabled libraries (only set when AllowedLibraryIDs is nil)
	LibrariesRestricted bool
	MaxContentRating    string
	MaxPlaybackQuality  string
	PolicyRevision      int64
	ProfileVerified     bool
}

// ResolveInput is the request input for resolving a viewer access scope.
type ResolveInput struct {
	UserID              int
	SessionID           string
	ProfileID           string
	ProfileToken        string
	SkipPINVerification bool
}

// ProfileTokenClaims are the claims embedded in a verified profile token.
type ProfileTokenClaims struct {
	UserID         int
	SessionID      string
	ProfileID      string
	PolicyRevision int64
}
