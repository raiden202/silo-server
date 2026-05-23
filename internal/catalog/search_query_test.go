package catalog

import "testing"

// TestNormalizeTitleForComparison_AmpersandAndEquivalence pins the contract
// that "&" and the word "and" are interchangeable. This mirrors the SQL
// function public.normalize_search_text() (migration 127). If the Go-side
// normalization drifts from the SQL one, ExactTitleHint will fail to match
// the title_normalized generated column for items whose stored title used
// the other form.
func TestNormalizeTitleForComparison_AmpersandAndEquivalence(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"ampersand collapses", "Law & Order", "law order"},
		{"and word strips", "Law and Order", "law order"},
		{"mixed punctuation", "Law and / & Order!", "law order"},
		{"and inside word preserved", "Random Sandwich", "random sandwich"},
		{"leading and stripped", "And Then There Were None", "then there were none"},
		{"trailing and stripped", "Order and", "order"},
		{"only and reduces to empty", "and", ""},
		{"single token kept", "and AND And", ""},
		{"consecutive and tokens", "rock and and roll", "rock roll"},
		{"acronym with ampersand", "S&P 500", "s p 500"},
		{"unicode letters", "Café & Crème", "café crème"},
		{"empty stays empty", "", ""},
		{"whitespace only", "   ", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeTitleForComparison(tc.in)
			if got != tc.want {
				t.Fatalf("normalizeTitleForComparison(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestParseSearchQuery_ExactTitleHint_StripsAnd ensures that the higher-level
// parser propagates the and-stripping normalization into ExactTitleHint, so
// callers building SQL with the hint get the form that matches title_normalized.
func TestParseSearchQuery_ExactTitleHint_StripsAnd(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Law & Order", "law order"},
		{"Law and Order", "law order"},
		{"\"Law and Order\" 2010", "law order"},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			parsed := parseSearchQuery(tc.in)
			if parsed.ExactTitleHint != tc.want {
				t.Fatalf("ExactTitleHint = %q, want %q (parsed = %+v)", parsed.ExactTitleHint, tc.want, parsed)
			}
		})
	}
}
