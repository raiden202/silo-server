package titleutil

import "testing"

func TestDeriveDefaultSortTitle(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"The Age of Adaline", "Age of Adaline, The"},
		{"The Office", "Office, The"},
		{"A Beautiful Mind", "Beautiful Mind, A"},
		{"An Inconvenient Truth", "Inconvenient Truth, An"},
		{"Inception", ""},
		{"the matrix", "matrix, The"},
		{"THE MATRIX", "MATRIX, The"},
		{"  The Matrix  ", "Matrix, The"},
		{"The", ""},
		{"A", ""},
		{"An", ""},
		{"Office, The", ""},
		{"American History X", ""},
		{"Theater", ""},
		{"", ""},
	}

	for _, tc := range tests {
		got := DeriveDefaultSortTitle(tc.title)
		if got != tc.want {
			t.Fatalf("DeriveDefaultSortTitle(%q) = %q, want %q", tc.title, got, tc.want)
		}
	}
}
