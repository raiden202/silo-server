package lang

import "testing"

func TestCanonical(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"   ", ""},
		{"en", "en"},
		{"EN", "en"},
		{" en ", "en"},
		{"eng", "en"},
		{"ENG", "en"},
		{"jpn", "ja"},
		{"ja", "ja"},
		{"fra", "fr"},
		{"fre", "fr"},
		{"fr", "fr"},
		{"deu", "de"},
		{"ger", "de"},
		{"zho", "zh"},
		{"chi", "zh"},
		{"nor", "no"},
		{"nob", "nb"},
		{"nb", "nb"},
		{"nn", "nn"},
		{"fr-CA", "fr"},
		{"en-US", "en"},
		{"pt-BR", "pt"},
		// Languages without a 2-letter form keep their 3-letter code.
		{"fil", "fil"},
		// Unrecognized inputs preserved as lowercase+trimmed.
		{"english", "english"},
		{"klingon", "klingon"},
		{"  Ja  ", "ja"},
	}
	for _, tc := range cases {
		got := Canonical(tc.in)
		if got != tc.want {
			t.Errorf("Canonical(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCanonicalCountry(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"  ", ""},
		{"US", "US"},
		{"us", "US"},
		{" us ", "US"},
		{"GB", "GB"},
		{"JP", "JP"},
		// Three-letter codes get canonicalized to alpha-2.
		{"USA", "US"},
		{"GBR", "GB"},
		{"JPN", "JP"},
		// Unrecognized stays uppercase+trimmed.
		{"United States", "UNITED STATES"},
		{"ZZ", "ZZ"},
	}
	for _, tc := range cases {
		got := CanonicalCountry(tc.in)
		if got != tc.want {
			t.Errorf("CanonicalCountry(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCanonicalCountries(t *testing.T) {
	cases := []struct {
		in   []string
		want []string
	}{
		{nil, nil},
		{[]string{}, []string{}},
		{[]string{"us", "GBR", "", "  jp  ", "ZZ"}, []string{"US", "GB", "JP", "ZZ"}},
		{[]string{"  ", ""}, []string{}},
	}
	for _, tc := range cases {
		got := CanonicalCountries(tc.in)
		if (got == nil) != (tc.want == nil) {
			t.Errorf("CanonicalCountries(%v) nil mismatch: got %v, want %v", tc.in, got, tc.want)
			continue
		}
		if len(got) != len(tc.want) {
			t.Errorf("CanonicalCountries(%v) = %v, want %v", tc.in, got, tc.want)
			continue
		}
		for i := range tc.want {
			if got[i] != tc.want[i] {
				t.Errorf("CanonicalCountries(%v)[%d] = %q, want %q", tc.in, i, got[i], tc.want[i])
			}
		}
	}
}
