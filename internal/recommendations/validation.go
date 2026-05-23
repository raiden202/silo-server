package recommendations

import "strings"

// validationResult holds the outcome of validating a candidate.
type validationResult struct {
	rejected  bool
	scoreMult float64 // 1.0 = no change, < 1.0 = penalty
}

// passesGenreGate returns true if the candidate shares at least one genre with
// the source, or if either has no genres (missing data should not penalize).
func passesGenreGate(sourceGenres, candidateGenres []string) bool {
	if len(sourceGenres) == 0 || len(candidateGenres) == 0 {
		return true
	}
	for _, sg := range sourceGenres {
		for _, cg := range candidateGenres {
			if strings.EqualFold(sg, cg) {
				return true
			}
		}
	}
	return false
}

// genreOverlapCount returns the number of shared genres (case-insensitive).
func genreOverlapCount(a, b []string) int {
	count := 0
	for _, ag := range a {
		for _, bg := range b {
			if strings.EqualFold(ag, bg) {
				count++
				break
			}
		}
	}
	return count
}

// titleWordOverlap computes the word overlap ratio between two titles.
// Strips leading English articles and lowercases both titles.
// Returns shared_words / min(len(sourceWords), len(candidateWords)).
func titleWordOverlap(sourceTitle, candidateTitle string) float64 {
	normalize := func(title string) []string {
		t := strings.ToLower(strings.TrimSpace(title))
		// Strip leading English articles.
		for _, article := range []string{"the ", "a ", "an "} {
			if strings.HasPrefix(t, article) {
				t = t[len(article):]
				break
			}
		}
		words := strings.Fields(t)
		if len(words) == 0 {
			return nil
		}
		return words
	}

	srcWords := normalize(sourceTitle)
	candWords := normalize(candidateTitle)
	if len(srcWords) == 0 || len(candWords) == 0 {
		return 0
	}

	// Build set from candidate words.
	candSet := make(map[string]bool, len(candWords))
	for _, w := range candWords {
		candSet[w] = true
	}

	shared := 0
	for _, w := range srcWords {
		if candSet[w] {
			shared++
		}
	}

	minLen := len(srcWords)
	if len(candWords) < minLen {
		minLen = len(candWords)
	}

	return float64(shared) / float64(minLen)
}

// validateCandidate runs the validation pipeline on a single candidate.
func validateCandidate(source, candidate *ItemMetadata, item ScoredItem) validationResult {
	overlap := titleWordOverlap(source.Title, candidate.Title)
	genreShared := genreOverlapCount(source.Genres, candidate.Genres)
	genreGatePass := passesGenreGate(source.Genres, candidate.Genres)

	// Step 1: Genre gate — reject zero genre overlap.
	if !genreGatePass {
		return validationResult{rejected: true}
	}

	// Step 2: Title overlap with exact match and different year — penalize.
	if overlap >= 1.0 && source.Year != candidate.Year && source.Year > 0 && candidate.Year > 0 {
		return validationResult{rejected: false, scoreMult: 0.5}
	}

	// Step 3: High title overlap with weak genre overlap (exactly 1 shared) — penalize.
	if overlap >= 0.5 && genreShared == 1 {
		return validationResult{rejected: false, scoreMult: 0.7}
	}

	_ = item // score available for future use
	return validationResult{rejected: false, scoreMult: 1.0}
}
