package jellycompat

import (
	"strings"
	"testing"
)

func TestAudioSelectionChanged(t *testing.T) {
	selected := 2
	session := &PlaybackSession{
		MediaSources: []PlaybackMediaSource{
			{ID: "src-a", SelectedAudioStreamIndex: &selected},
			{ID: "src-b", SelectedAudioStreamIndex: nil},
		},
	}

	tests := []struct {
		name          string
		session       *PlaybackSession
		mediaSourceID string
		incoming      int
		want          bool
	}{
		{"same index on known source", session, "src-a", 2, false},
		{"different index on known source", session, "src-a", 3, true},
		{"nil current on known source", session, "src-b", 2, true},
		{"unknown media source id", session, "src-missing", 2, true},
		{"empty media source id uses first match", session, "", 2, false},
		{"nil session", nil, "src-a", 2, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := audioSelectionChanged(tc.session, tc.mediaSourceID, tc.incoming)
			if got != tc.want {
				t.Errorf("audioSelectionChanged(%q, %d) = %v, want %v", tc.mediaSourceID, tc.incoming, got, tc.want)
			}
		})
	}
}

func TestGenerateFullManifest_HLSVersionForResumeStartTag(t *testing.T) {
	cases := []struct {
		name        string
		fmp4        bool
		startOffset float64
		wantVersion string
		wantStart   bool
	}{
		{"ts no resume", false, 0, "#EXT-X-VERSION:3", false},
		{"ts with resume", false, 5.5, "#EXT-X-VERSION:6", true},
		{"fmp4 no resume", true, 0, "#EXT-X-VERSION:7", false},
		{"fmp4 with resume", true, 5.5, "#EXT-X-VERSION:7", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(generateFullManifest(60, 2, tc.fmp4, tc.startOffset))
			if !strings.Contains(got, tc.wantVersion+"\n") {
				t.Fatalf("missing %s; manifest:\n%s", tc.wantVersion, got)
			}
			hasStart := strings.Contains(got, "#EXT-X-START:")
			if hasStart != tc.wantStart {
				t.Fatalf("EXT-X-START presence = %v, want %v; manifest:\n%s", hasStart, tc.wantStart, got)
			}
		})
	}
}

func TestRewriteManifest_PreservesPlaybackAndMediaSourceIDs(t *testing.T) {
	manifest := strings.Join([]string{
		"#EXTM3U",
		"#EXT-X-VERSION:7",
		"#EXT-X-MAP:URI=\"init.mp4\"",
		"#EXTINF:2.000000,",
		"seg_00000.m4s",
		"#EXTINF:2.000000,",
		"stream.m3u8",
		"",
	}, "\n")

	got := string(rewriteManifest([]byte(manifest), "item-1", "play-1", "source-1"))

	if !strings.Contains(got, "#EXT-X-MAP:URI=\"/Videos/item-1/hls/play-1/init.mp4?MediaSourceId=source-1&PlaySessionId=play-1\"") {
		t.Fatalf("expected init segment to include media and playback session ids, got:\n%s", got)
	}
	if !strings.Contains(got, "/Videos/item-1/hls/play-1/seg_00000.m4s?MediaSourceId=source-1&PlaySessionId=play-1") {
		t.Fatalf("expected media segment to include media and playback session ids, got:\n%s", got)
	}
	if !strings.Contains(got, "/Videos/item-1/hls/play-1/stream.m3u8?MediaSourceId=source-1&PlaySessionId=play-1") {
		t.Fatalf("expected nested manifest to include media and playback session ids, got:\n%s", got)
	}
}
