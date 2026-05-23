package libraryingest

import "context"

// ProgressUpdate captures in-flight scan state suitable for logs, persisted
// scan rows, and realtime admin views.
type ProgressUpdate struct {
	Phase           string
	Message         string
	CurrentScope    string
	TotalFiles      int
	FilesDiscovered int
	FilesProcessed  int
	New             int
	Updated         int
	Unchanged       int
	Errors          int
	MatchedFiles    int
	RetriedItems    int
}

type progressReporterKey struct{}

type progressReporter func(ProgressUpdate)

// WithProgressReporter attaches an optional in-flight progress reporter to the
// ingest context.
func WithProgressReporter(ctx context.Context, reporter func(ProgressUpdate)) context.Context {
	if ctx == nil || reporter == nil {
		return ctx
	}
	return context.WithValue(ctx, progressReporterKey{}, progressReporter(reporter))
}

func reportProgress(ctx context.Context, update ProgressUpdate) {
	if ctx == nil {
		return
	}
	reporter, _ := ctx.Value(progressReporterKey{}).(progressReporter)
	if reporter == nil {
		return
	}
	reporter(update)
}
