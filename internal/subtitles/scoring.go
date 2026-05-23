// internal/subtitles/scoring.go
package subtitles

import (
	"fmt"
	"strings"
)

// Scoring point values from spec.
const (
	episodeTitlePoints   = 120
	episodeYearPoints    = 60
	episodeSeasonPoints  = 20
	episodeEpisodePoints = 20
	movieTitlePoints     = 60
	movieYearPoints      = 30
	groupPoints          = 15
	sourcePoints         = 8
	audioCodecPoints     = 3
	resolutionPoints     = 2
	videoCodecPoints     = 2
)

// ScoreResult scores a subtitle result against media metadata.
// isEpisode should be true for TV episodes (season > 0).
// Returns a score 0-100.
func ScoreResult(result SubtitleResult, req SearchRequest) float64 {
	isEpisode := req.Season > 0

	subInfo := ParseReleaseInfo(result.ReleaseName)
	subName := normalizeTitle(result.ReleaseName)

	var matched, maxPoints float64

	if isEpisode {
		maxPoints += episodeTitlePoints
		if titleMatches(subName, req.Title) {
			matched += episodeTitlePoints
		}
		if req.Year > 0 {
			maxPoints += episodeYearPoints
			if strings.Contains(subName, fmt.Sprintf("%d", req.Year)) {
				matched += episodeYearPoints
			}
		}
		if req.Season > 0 {
			maxPoints += episodeSeasonPoints
			if seasonMatches(subName, req.Season) {
				matched += episodeSeasonPoints
			}
		}
		if req.Episode > 0 {
			maxPoints += episodeEpisodePoints
			if episodeMatches(subName, req.Episode) {
				matched += episodeEpisodePoints
			}
		}
	} else {
		maxPoints += movieTitlePoints
		if titleMatches(subName, req.Title) {
			matched += movieTitlePoints
		}
		if req.Year > 0 {
			maxPoints += movieYearPoints
			if strings.Contains(subName, fmt.Sprintf("%d", req.Year)) {
				matched += movieYearPoints
			}
		}
	}

	// MediaInfo fields always count toward maxPoints when present in the request,
	// so that missing metadata in the subtitle name is penalized.
	maxPoints += groupPoints + sourcePoints + audioCodecPoints + resolutionPoints + videoCodecPoints

	if req.MediaInfo != nil {
		if req.MediaInfo.ReleaseGroup != "" && strings.EqualFold(subInfo.ReleaseGroup, req.MediaInfo.ReleaseGroup) {
			matched += groupPoints
		}
		if req.MediaInfo.Source != "" && strings.EqualFold(subInfo.Source, req.MediaInfo.Source) {
			matched += sourcePoints
		}
		if req.MediaInfo.AudioCodec != "" && strings.EqualFold(subInfo.AudioCodec, req.MediaInfo.AudioCodec) {
			matched += audioCodecPoints
		}
		if req.MediaInfo.Resolution != "" && strings.EqualFold(subInfo.Resolution, req.MediaInfo.Resolution) {
			matched += resolutionPoints
		}
		if req.MediaInfo.VideoCodec != "" && strings.EqualFold(subInfo.VideoCodec, req.MediaInfo.VideoCodec) {
			matched += videoCodecPoints
		}
	}

	if maxPoints == 0 {
		return 0
	}
	return (matched / maxPoints) * 100
}

func normalizeTitle(s string) string {
	return strings.ToLower(strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(s))
}

func titleMatches(subName, title string) bool {
	if title == "" {
		return false
	}
	return strings.Contains(subName, strings.ToLower(title))
}

func seasonMatches(subName string, season int) bool {
	patterns := []string{
		strings.ToLower(fmt.Sprintf("s%02d", season)),
		strings.ToLower(fmt.Sprintf("s%d", season)),
		strings.ToLower(fmt.Sprintf("season %d", season)),
	}
	for _, p := range patterns {
		if strings.Contains(subName, p) {
			return true
		}
	}
	return false
}

func episodeMatches(subName string, episode int) bool {
	patterns := []string{
		strings.ToLower(fmt.Sprintf("e%02d", episode)),
		strings.ToLower(fmt.Sprintf("e%d", episode)),
	}
	for _, p := range patterns {
		if strings.Contains(subName, p) {
			return true
		}
	}
	return false
}
