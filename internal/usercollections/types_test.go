package usercollections

import "testing"

func TestNormalizeMDBListURL(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"  ", ""},
		{"https://mdblist.com/lists/quickflix/wwe", "https://mdblist.com/lists/quickflix/wwe/json"},
		{"https://mdblist.com/lists/quickflix/wwe/", "https://mdblist.com/lists/quickflix/wwe/json"},
		{"https://mdblist.com/lists/quickflix/wwe/json", "https://mdblist.com/lists/quickflix/wwe/json"},
		{"https://mdblist.com/lists/quickflix/wwe/json/", "https://mdblist.com/lists/quickflix/wwe/json"},
		{"  https://mdblist.com/lists/quickflix/wwe  ", "https://mdblist.com/lists/quickflix/wwe/json"},
	}
	for _, tc := range cases {
		if got := NormalizeMDBListURL(tc.in); got != tc.want {
			t.Errorf("NormalizeMDBListURL(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
