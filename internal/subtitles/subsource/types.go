// internal/subtitles/subsource/types.go
package subsource

import "time"

// movieSearchResponse is the response from GET /movies/search.
type movieSearchResponse struct {
	Success bool          `json:"success"`
	Data    []movieResult `json:"data"`
}

type movieResult struct {
	MovieID       int    `json:"movieId"`
	Title         string `json:"title"`
	Type          string `json:"type"` // "movie" or "tvseries"
	ReleaseYear   int    `json:"releaseYear"`
	IMDbID        string `json:"imdbId"`
	TMDbID        string `json:"tmdbId"`
	Season        *int   `json:"season"`
	SubtitleCount int    `json:"subtitleCount"`
}

// subtitleListResponse is the response from GET /subtitles.
type subtitleListResponse struct {
	Success    bool            `json:"success"`
	Data       []subtitleEntry `json:"data"`
	Pagination pagination      `json:"pagination"`
}

type pagination struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
	Total int `json:"total"`
	Pages int `json:"pages"`
}

type subtitleEntry struct {
	SubtitleID      int            `json:"subtitleId"`
	MovieID         int            `json:"movieId"`
	Language        string         `json:"language"`
	ReleaseInfo     []string       `json:"releaseInfo"`
	Commentary      string         `json:"commentary"`
	Files           int            `json:"files"`
	Size            int            `json:"size"`
	HearingImpaired bool           `json:"hearingImpaired"`
	ForeignParts    bool           `json:"foreignParts"`
	Framerate       string         `json:"framerate"`
	ProductionType  string         `json:"productionType"`
	ReleaseType     string         `json:"releaseType"`
	Downloads       int            `json:"downloads"`
	Rating          subtitleRating `json:"rating"`
	UploadedAt      time.Time      `json:"createdAt"`
}

type subtitleRating struct {
	Good  int `json:"good"`
	Bad   int `json:"bad"`
	Total int `json:"total"`
}
