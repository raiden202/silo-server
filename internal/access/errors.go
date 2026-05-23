package access

import "errors"

var (
	ErrProfileNotFound   = errors.New("profile not found")
	ErrProfileUnverified = errors.New("profile unverified")
)
