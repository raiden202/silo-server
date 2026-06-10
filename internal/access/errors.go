package access

import "errors"

var (
	ErrProfileNotFound   = errors.New("profile not found")
	ErrProfileUnverified = errors.New("profile unverified")
	// ErrUserDisabled is returned when the viewer's account is disabled.
	// Scope resolution re-loads the user on every request, so disabling an
	// account denies content access on the next request for every surface
	// that resolves viewer scope (native API and Jellyfin compat alike).
	ErrUserDisabled = errors.New("user account disabled")
)
