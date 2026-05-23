package nfo

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestFindNFO_FilePathFallsBackToDirectoryLevelSidecar(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "Blade Runner (1982).mkv")
	if err := os.WriteFile(filePath, []byte("video"), 0o644); err != nil {
		t.Fatalf("WriteFile(file) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "movie.nfo"), []byte("dir"), 0o644); err != nil {
		t.Fatalf("WriteFile(movie.nfo) error = %v", err)
	}

	want := filepath.Join(dir, "movie.nfo")
	if got := findNFO([]string{filePath}); got != want {
		t.Fatalf("findNFO(file path) = %q, want %q", got, want)
	}
}

func TestFindNFO_DirectoryPathUsesDirectoryLevelSidecar(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	want := filepath.Join(dir, "movie.nfo")
	if err := os.WriteFile(want, []byte("dir"), 0o644); err != nil {
		t.Fatalf("WriteFile(movie.nfo) error = %v", err)
	}

	if got := findNFO([]string{dir}); got != want {
		t.Fatalf("findNFO(directory path) = %q, want %q", got, want)
	}
}

func TestFindNFO_FilePathUsesBasenameMatchedNFO(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "Blade Runner (1982).mkv")
	want := filepath.Join(dir, "Blade Runner (1982).nfo")
	if err := os.WriteFile(filePath, []byte("video"), 0o644); err != nil {
		t.Fatalf("WriteFile(file) error = %v", err)
	}
	if err := os.WriteFile(want, []byte("file"), 0o644); err != nil {
		t.Fatalf("WriteFile(file.nfo) error = %v", err)
	}

	if got := findNFO([]string{filePath}); got != want {
		t.Fatalf("findNFO(file path) = %q, want %q", got, want)
	}
}

func TestFindNFO_ExtensionlessFilePathUsesDirectoryLevelSidecar(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "Blade Runner (1982)")
	want := filepath.Join(dir, "movie.nfo")
	if err := os.WriteFile(filePath, []byte("video"), 0o644); err != nil {
		t.Fatalf("WriteFile(file) error = %v", err)
	}
	if err := os.WriteFile(want, []byte("dir"), 0o644); err != nil {
		t.Fatalf("WriteFile(movie.nfo) error = %v", err)
	}

	if got := findNFO([]string{filePath}); got != want {
		t.Fatalf("findNFO(extensionless file path) = %q, want %q", got, want)
	}
}

func TestFindNFO_ExtensionlessFilePathUsesBasenameMatchedNFO(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "Blade Runner (1982)")
	want := filepath.Join(dir, "Blade Runner (1982).nfo")
	if err := os.WriteFile(filePath, []byte("video"), 0o644); err != nil {
		t.Fatalf("WriteFile(file) error = %v", err)
	}
	if err := os.WriteFile(want, []byte("file"), 0o644); err != nil {
		t.Fatalf("WriteFile(file.nfo) error = %v", err)
	}

	if got := findNFO([]string{filePath}); got != want {
		t.Fatalf("findNFO(extensionless file path) = %q, want %q", got, want)
	}
}

func TestNFOCandidatesForMissingDottedDirectoryPathIncludeDirectoryLevelSidecars(t *testing.T) {
	t.Parallel()

	path := filepath.Join("/media/shows", "Mr. Robot")
	got := nfoCandidatesForPath(path)

	wantMovie := filepath.Join(path, "movie.nfo")
	wantTV := filepath.Join(path, "tvshow.nfo")
	if !slices.Contains(got, wantMovie) {
		t.Fatalf("nfoCandidatesForPath(%q) missing %q in %#v", path, wantMovie, got)
	}
	if !slices.Contains(got, wantTV) {
		t.Fatalf("nfoCandidatesForPath(%q) missing %q in %#v", path, wantTV, got)
	}
}
