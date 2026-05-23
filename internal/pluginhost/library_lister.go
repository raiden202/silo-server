package pluginhost

import "context"

// LibraryDataSource produces library records, optionally scoped to a user.
// In production, an adapter over the existing libraries data access (e.g.
// catalog.FolderRepository) implements this. In tests, fakes do.
type LibraryDataSource interface {
	List(ctx context.Context, userID string) ([]LibraryRecord, error)
}

// LibraryDataSourceFunc adapts a plain function to LibraryDataSource so
// callers can pass a closure without defining a struct.
type LibraryDataSourceFunc func(ctx context.Context, userID string) ([]LibraryRecord, error)

func (f LibraryDataSourceFunc) List(ctx context.Context, userID string) ([]LibraryRecord, error) {
	return f(ctx, userID)
}

// libraryLister is the LibraryLister consumed by RuntimeHostServer. It just
// forwards to the underlying data source; the value of this layer is the
// interface boundary, which keeps RuntimeHostServer from depending on
// catalog/FolderRepository directly.
type libraryLister struct {
	src LibraryDataSource
}

// NewLibraryLister returns a LibraryLister backed by the given data source.
func NewLibraryLister(src LibraryDataSource) LibraryLister {
	return &libraryLister{src: src}
}

func (l *libraryLister) ListLibraries(ctx context.Context, userID string) ([]LibraryRecord, error) {
	return l.src.List(ctx, userID)
}
