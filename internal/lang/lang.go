// Package lang canonicalizes ISO 639 language codes and ISO 3166-1 country
// codes at ingest write sites so equivalent values ("en"/"eng"/"ENG")
// collapse to a single stored form.
package lang

import (
	"strings"

	"golang.org/x/text/language"
)

// Canonical returns the ISO 639-1 lowercase 2-letter form of value, or the
// 3-letter form for languages without a 2-letter equivalent (e.g. "fil").
// Unparseable inputs are returned trimmed and lowercased verbatim so we
// never silently drop data.
func Canonical(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return ""
	}
	tag, err := language.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	base, conf := tag.Base()
	if conf == language.No {
		return trimmed
	}
	return strings.ToLower(base.String())
}

// CanonicalCountry returns the ISO 3166-1 alpha-2 uppercase form. Unparseable
// inputs are returned trimmed and uppercased verbatim.
func CanonicalCountry(value string) string {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		return ""
	}
	region, err := language.ParseRegion(trimmed)
	if err != nil {
		return trimmed
	}
	return region.String()
}

// CanonicalCountries returns a copy of values with each entry canonicalized
// and empties dropped. Preserves nil so callers can keep the SQL NULL
// distinction from an empty array.
func CanonicalCountries(values []string) []string {
	if values == nil {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		c := CanonicalCountry(v)
		if c != "" {
			out = append(out, c)
		}
	}
	return out
}
