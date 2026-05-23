package catalog

import (
	"strconv"
	"strings"
	"unicode"
)

type parsedSearchQuery struct {
	Raw            string
	Text           string
	Phrase         string
	ExactTitleHint string
	Year           *int
}

func parseSearchQuery(raw string) parsedSearchQuery {
	trimmed := collapseSearchWhitespace(strings.TrimSpace(raw))
	phrase, remainder := extractBalancedPhrase(trimmed)
	year, remainder := extractYearHint(remainder, phrase != "")

	parts := make([]string, 0, 2)
	if phrase != "" {
		parts = append(parts, phrase)
	}
	if remainder != "" {
		parts = append(parts, remainder)
	}

	text := collapseSearchWhitespace(strings.Join(parts, " "))
	if text == "" {
		text = collapseSearchWhitespace(strings.ReplaceAll(trimmed, "\"", " "))
	}

	return parsedSearchQuery{
		Raw:            raw,
		Text:           text,
		Phrase:         phrase,
		ExactTitleHint: normalizeTitleForComparison(firstNonEmptySearchValue(phrase, text)),
		Year:           year,
	}
}

func extractBalancedPhrase(input string) (string, string) {
	start := strings.Index(input, "\"")
	if start == -1 {
		return "", input
	}

	end := strings.Index(input[start+1:], "\"")
	if end == -1 {
		return "", collapseSearchWhitespace(strings.ReplaceAll(input, "\"", " "))
	}

	end += start + 1
	phrase := collapseSearchWhitespace(input[start+1 : end])
	remainder := collapseSearchWhitespace(strings.Join([]string{
		input[:start],
		input[end+1:],
	}, " "))
	return phrase, remainder
}

func extractYearHint(input string, hasPhrase bool) (*int, string) {
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return nil, ""
	}

	for i := len(fields) - 1; i >= 0; i-- {
		year, ok := parseYearToken(fields[i])
		if !ok {
			continue
		}
		if len(fields) == 1 && !hasPhrase {
			return nil, collapseSearchWhitespace(input)
		}

		remaining := append([]string{}, fields[:i]...)
		remaining = append(remaining, fields[i+1:]...)
		return &year, collapseSearchWhitespace(strings.Join(remaining, " "))
	}

	return nil, collapseSearchWhitespace(input)
}

func parseYearToken(token string) (int, bool) {
	if len(token) != 4 {
		return 0, false
	}
	year, err := strconv.Atoi(token)
	if err != nil {
		return 0, false
	}
	if year < 1900 || year > 2100 {
		return 0, false
	}
	return year, true
}

// normalizeTitleForComparison must stay in lockstep with the SQL function
// public.normalize_search_text (migration 127) and the title_normalized
// generated column. Mismatches between Go and SQL normalization produce
// asymmetric search results (Go-computed ExactTitleHint failing to match a
// row whose title_normalized has the same logical content).
func normalizeTitleForComparison(input string) string {
	var b strings.Builder
	b.Grow(len(input))

	for _, r := range input {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteByte(' ')
		}
	}

	return stripStandaloneAndTokens(collapseSearchWhitespace(b.String()))
}

// stripStandaloneAndTokens drops the standalone token "and" from a
// whitespace-separated lowercase string. Together with the alphanumeric
// pass above, this makes "&" and the word "and" interchangeable: both
// "Law & Order" and "Law and Order" reduce to "law order".
func stripStandaloneAndTokens(input string) string {
	if input == "" {
		return ""
	}
	fields := strings.Fields(input)
	filtered := fields[:0]
	for _, f := range fields {
		if f == "and" {
			continue
		}
		filtered = append(filtered, f)
	}
	return strings.Join(filtered, " ")
}

func collapseSearchWhitespace(input string) string {
	return strings.Join(strings.Fields(input), " ")
}

func firstNonEmptySearchValue(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
