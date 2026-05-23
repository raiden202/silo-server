package metadata

import (
	"regexp"
	"strings"
	"time"

	"github.com/Silo-Server/silo-server/internal/models"
)

const EpisodeRecentStillWindow = 45 * 24 * time.Hour

var episodePlaceholderTitlePattern = regexp.MustCompile(`(?i)^(tba|tbd|episode\s+\d+)$`)

func IsEpisodePlaceholderTitle(title string) bool {
	title = strings.TrimSpace(title)
	if title == "" {
		return false
	}
	return episodePlaceholderTitlePattern.MatchString(title)
}

func EpisodeHasIncompleteMetadata(ep *models.Episode, now time.Time) bool {
	if ep == nil {
		return false
	}
	if strings.TrimSpace(ep.Title) == "" {
		return true
	}
	if IsEpisodePlaceholderTitle(ep.Title) {
		return true
	}
	if strings.TrimSpace(ep.Overview) == "" {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(ep.MetadataSource), "scanner_fallback") {
		return true
	}
	if ep.AirDate != nil &&
		!ep.AirDate.Before(now.Add(-EpisodeRecentStillWindow)) &&
		strings.TrimSpace(ep.StillPath) == "" {
		return true
	}
	return false
}
