package nfo

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/Silo-Server/silo-server/internal/metadata"
)

// Provider reads metadata from NFO sidecar files.
type Provider struct{}

func NewProvider() *Provider { return &Provider{} }

func (p *Provider) Slug() string       { return "nfo" }
func (p *Provider) Name() string       { return "NFO Files" }
func (p *Provider) ForTypes() []string { return []string{"movie", "series"} }

// Search extracts external IDs from NFO for matching.
func (p *Provider) Search(ctx context.Context, query metadata.SearchQuery) ([]metadata.SearchResult, error) {
	nfoPath := findNFOForQuery(query)
	if nfoPath == "" {
		return nil, nil
	}
	data, err := os.ReadFile(nfoPath)
	if err != nil {
		return nil, nil // NFO read errors are not fatal
	}
	parsed, err := parseNFOData(data)
	if err != nil {
		return nil, nil // NFO parse errors are not fatal
	}
	if query.ContentType != "" && parsed.Type != "" && parsed.Type != query.ContentType {
		return nil, nil
	}
	ids := make(map[string]string)
	if parsed.TmdbID != "" {
		ids["tmdb"] = parsed.TmdbID
	}
	if parsed.ImdbID != "" {
		ids["imdb"] = parsed.ImdbID
	}
	if parsed.TvdbID != "" {
		ids["tvdb"] = parsed.TvdbID
	}
	if len(ids) == 0 && parsed.Title == "" {
		return nil, nil
	}
	return []metadata.SearchResult{{
		Name:        parsed.Title,
		Year:        parsed.Year,
		ProviderIDs: ids,
		Provider:    p.Slug(),
	}}, nil
}

// GetMetadata parses full metadata from NFO file.
func (p *Provider) GetMetadata(ctx context.Context, req metadata.MetadataRequest) (*metadata.MetadataResult, error) {
	nfoPath := findNFOForRequest(req)
	if nfoPath == "" {
		return &metadata.MetadataResult{}, nil
	}
	data, err := os.ReadFile(nfoPath)
	if err != nil {
		return &metadata.MetadataResult{}, nil
	}
	parsed, err := parseNFOData(data)
	if err != nil {
		return &metadata.MetadataResult{}, nil
	}
	result := &metadata.MetadataResult{
		HasMetadata: true,
		Title:       parsed.Title,
		Year:        parsed.Year,
		Overview:    parsed.Overview,
		ProviderIDs: make(map[string]string),
	}
	if parsed.TmdbID != "" {
		result.ProviderIDs["tmdb"] = parsed.TmdbID
	}
	if parsed.ImdbID != "" {
		result.ProviderIDs["imdb"] = parsed.ImdbID
	}
	if parsed.TvdbID != "" {
		result.ProviderIDs["tvdb"] = parsed.TvdbID
	}
	return result, nil
}

func findNFOForQuery(query metadata.SearchQuery) string {
	candidatePaths := candidateSidecarPaths(
		query.FilePath,
		query.RepresentativeFilePath,
		query.AllGroupFilePaths,
		query.PrimarySidecarSearchPaths,
		query.ProviderIDs["_filepath"],
	)
	return findNFO(candidatePaths)
}

func findNFOForRequest(req metadata.MetadataRequest) string {
	candidatePaths := candidateSidecarPaths(
		req.FilePath,
		req.RepresentativeFilePath,
		req.AllGroupFilePaths,
		req.PrimarySidecarSearchPaths,
	)
	return findNFO(candidatePaths)
}

func candidateSidecarPaths(primary string, representative string, groupFiles []string, searchPaths []string, extras ...string) []string {
	candidates := make([]string, 0, 2+len(groupFiles)+len(searchPaths)+len(extras))
	for _, path := range []string{primary, representative} {
		if strings.TrimSpace(path) != "" {
			candidates = append(candidates, path)
		}
	}
	candidates = append(candidates, groupFiles...)
	candidates = append(candidates, searchPaths...)
	candidates = append(candidates, extras...)
	return compactNFOPaths(candidates)
}

func compactNFOPaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		clean := filepath.Clean(strings.TrimSpace(path))
		if clean == "" || clean == "." {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	return out
}

// findNFO locates the best candidate NFO file across media-file and directory
// search paths. File paths check legacy directory-level sidecars first, then a
// basename-matched sidecar in the same directory.
func findNFO(paths []string) string {
	candidates := make([]string, 0, len(paths)*3)
	for _, path := range paths {
		candidates = append(candidates, nfoCandidatesForPath(path)...)
	}
	candidates = compactNFOPaths(candidates)
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func nfoCandidatesForPath(path string) []string {
	if path == "" {
		return nil
	}
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return directoryLevelNFOCandidates(path)
		}
		return fileLevelNFOCandidates(path)
	}
	return append(fileLevelNFOCandidates(path), directoryLevelNFOCandidates(path)...)
}

func fileLevelNFOCandidates(path string) []string {
	ext := filepath.Ext(path)
	dir := filepath.Dir(path)
	base := strings.TrimSuffix(filepath.Base(path), ext)
	return []string{
		filepath.Join(dir, "movie.nfo"),
		filepath.Join(dir, "tvshow.nfo"),
		filepath.Join(dir, base+".nfo"),
	}
}

func directoryLevelNFOCandidates(path string) []string {
	return []string{
		filepath.Join(path, "movie.nfo"),
		filepath.Join(path, "tvshow.nfo"),
	}
}
