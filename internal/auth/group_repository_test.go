package auth

import "testing"

func TestSlugifyGroupName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Family", "family"},
		{"Power Users", "power-users"},
		{"  Trimmed  ", "trimmed"},
		{"Ünïcode & Symbols!", "unicode-symbols"},
		{"--multi---dash--", "multi-dash"},
	}
	for _, tc := range cases {
		if got := slugifyGroupName(tc.in); got != tc.want {
			t.Errorf("slugifyGroupName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
