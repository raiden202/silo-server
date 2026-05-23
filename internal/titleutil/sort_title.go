package titleutil

import (
	"strings"
	"unicode"
)

// DeriveDefaultSortTitle returns a Plex-style sort title when title begins with
// a leading English article (The, A, An). The article moves to a comma suffix
// so "The Office" becomes "Office, The". Returns "" when no derivation applies.
func DeriveDefaultSortTitle(title string) string {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return ""
	}

	spaceIdx := strings.IndexFunc(trimmed, unicode.IsSpace)
	var firstWord, rest string
	if spaceIdx < 0 {
		firstWord = trimmed
		rest = ""
	} else {
		firstWord = trimmed[:spaceIdx]
		rest = strings.TrimSpace(trimmed[spaceIdx:])
	}

	switch strings.ToLower(firstWord) {
	case "the":
		if rest == "" {
			return ""
		}
		return rest + ", The"
	case "a":
		if rest == "" {
			return ""
		}
		return rest + ", A"
	case "an":
		if rest == "" {
			return ""
		}
		return rest + ", An"
	default:
		return ""
	}
}
