package recommendations

import (
	"sort"
	"strings"
)

// FilterAndRankGenreMatches keeps only candidates that match at least one
// selected genre and orders them by shared-genre count before similarity.
func FilterAndRankGenreMatches(items []ScoredItem, genres []string, genreMap map[string][]string) []ScoredItem {
	if len(items) == 0 {
		return items
	}

	if len(genres) == 0 {
		sort.Slice(items, func(i, j int) bool {
			if items[i].Score != items[j].Score {
				return items[i].Score > items[j].Score
			}
			return items[i].MediaItemID < items[j].MediaItemID
		})
		return items
	}

	type rankedCandidate struct {
		item       ScoredItem
		matchCount int
	}

	selected := make(map[string]struct{}, len(genres))
	for _, genre := range genres {
		selected[strings.ToLower(strings.TrimSpace(genre))] = struct{}{}
	}

	ranked := make([]rankedCandidate, 0, len(items))
	for _, item := range items {
		matchCount := genreMatchCount(genreMap[item.MediaItemID], selected)
		if matchCount == 0 {
			continue
		}
		ranked = append(ranked, rankedCandidate{
			item:       item,
			matchCount: matchCount,
		})
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].matchCount != ranked[j].matchCount {
			return ranked[i].matchCount > ranked[j].matchCount
		}
		if ranked[i].item.Score != ranked[j].item.Score {
			return ranked[i].item.Score > ranked[j].item.Score
		}
		return ranked[i].item.MediaItemID < ranked[j].item.MediaItemID
	})

	filtered := make([]ScoredItem, 0, len(ranked))
	for _, candidate := range ranked {
		filtered = append(filtered, candidate.item)
	}
	return filtered
}

func genreMatchCount(candidateGenres []string, selected map[string]struct{}) int {
	if len(candidateGenres) == 0 || len(selected) == 0 {
		return 0
	}

	matchCount := 0
	seen := make(map[string]struct{}, len(candidateGenres))
	for _, genre := range candidateGenres {
		key := strings.ToLower(strings.TrimSpace(genre))
		if key == "" {
			continue
		}
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		if _, match := selected[key]; match {
			matchCount++
		}
	}

	return matchCount
}
