package playback

import "errors"

// Sentinel errors for playback operations.
var (
	ErrNoVersions        = errors.New("no file versions available")
	ErrSessionNotFound   = errors.New("playback session not found")
	ErrTooManyStreams    = errors.New("too many concurrent streams")
	ErrTooManyTranscodes = errors.New("too many concurrent transcodes")
	ErrFileNotFound      = errors.New("media file not found")
	ErrTranscodeFailed   = errors.New("transcode process failed")
	ErrSegmentNotFound   = errors.New("segment not found")
	ErrManifestNotReady  = errors.New("manifest not ready")
)
