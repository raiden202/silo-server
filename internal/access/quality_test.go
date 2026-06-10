package access

import "testing"

func TestQuality_Order(t *testing.T) {
	if CompareQuality("480p", "720p") >= 0 {
		t.Fatal("expected 480p < 720p")
	}
	if CompareQuality("720p", "1080p") >= 0 {
		t.Fatal("expected 720p < 1080p")
	}
	if CompareQuality("1080p", "2160p") >= 0 {
		t.Fatal("expected 1080p < 2160p")
	}
	if CompareQuality("2160p", "4320p") >= 0 {
		t.Fatal("expected 2160p < 4320p")
	}
}

func TestQuality_FitsWithinCeiling(t *testing.T) {
	if !QualityAllowed("720p", "1080p") {
		t.Fatal("expected 720p under 1080p ceiling")
	}
	if QualityAllowed("2160p", "1080p") {
		t.Fatal("expected 2160p over 1080p ceiling")
	}
	if QualityAllowed("4320p", "2160p") {
		t.Fatal("expected 4320p over 2160p ceiling")
	}
}

func TestParsePlaybackQualityPreset(t *testing.T) {
	tests := map[string]string{
		"":         "",
		"any":      "",
		"standard": "1080p",
		"720p":     "1080p",
		"1080p":    "1080p",
		"4k":       "2160p",
		"2160p":    "2160p",
	}

	for input, want := range tests {
		got, ok := ParsePlaybackQualityPreset(input)
		if !ok {
			t.Fatalf("ParsePlaybackQualityPreset(%q) returned ok=false", input)
		}
		if got != want {
			t.Fatalf("ParsePlaybackQualityPreset(%q) = %q, want %q", input, got, want)
		}
	}

	if _, ok := ParsePlaybackQualityPreset("banana"); ok {
		t.Fatal("expected invalid preset to return ok=false")
	}
}

func TestMaxQuality(t *testing.T) {
	cases := []struct {
		a, b, want string
	}{
		{"", "", ""},
		{"", "1080p", ""}, // unrestricted beats any ceiling
		{"2160p", "", ""},
		{"1080p", "2160p", "2160p"},
		{"2160p", "1080p", "2160p"},
		{"1080p", "1080p", "1080p"},
		{"720p", "480p", "1080p"}, // presets normalize: 720p/480p both -> 1080p (standard)
	}
	for _, tc := range cases {
		if got := MaxQuality(tc.a, tc.b); got != tc.want {
			t.Errorf("MaxQuality(%q, %q) = %q, want %q", tc.a, tc.b, got, tc.want)
		}
	}
}
