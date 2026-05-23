package intromarkers

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestParseSilenceDetectOutput(t *testing.T) {
	output := []byte(`
[silencedetect @ 0x1] silence_start: 19.351
[silencedetect @ 0x1] silence_end: 29.319 | silence_duration: 9.968
malformed silence_start: nope
[silencedetect @ 0x1] silence_start: 31
[silencedetect @ 0x1] silence_end: 31.5 | silence_duration: 0.5
`)
	intervals := parseSilenceDetectOutput(output, 165)
	if len(intervals) != 2 {
		t.Fatalf("expected two intervals, got %d", len(intervals))
	}
	if intervals[0].Start != 184.351 || intervals[0].End != 194.319 {
		t.Fatalf("unexpected first interval: %+v", intervals[0])
	}
	if intervals[1].Start != 196 || intervals[1].End != 196.5 {
		t.Fatalf("unexpected second interval: %+v", intervals[1])
	}
}

func TestParseSilenceDetectOutputEmpty(t *testing.T) {
	intervals := parseSilenceDetectOutput([]byte("no silence here"), 100)
	if len(intervals) != 0 {
		t.Fatalf("expected no intervals, got %d", len(intervals))
	}
}

func TestSilenceBoundaryRefinerAppliesFirstUsableSilence(t *testing.T) {
	ffmpeg := writeFakeFFmpeg(t, `
echo "[silencedetect @ 0x1] silence_start: 1" >&2
echo "[silencedetect @ 0x1] silence_end: 1.2 | silence_duration: 0.2" >&2
echo "[silencedetect @ 0x1] silence_start: 7" >&2
echo "[silencedetect @ 0x1] silence_end: 8 | silence_duration: 1" >&2
exit 0
`)
	refiner := NewSilenceBoundaryRefiner(DefaultConfig(ffmpeg))
	segment := Segment{Start: 60, End: 120, Confidence: 0.95, Algorithm: ChapterAlgorithm}
	candidate := Candidate{FileID: 1, FilePath: "/tmp/media.mkv", DurationSeconds: 1200}

	refined, ok, err := refiner.RefineChapterEnd(context.Background(), candidate, segment)
	if err != nil {
		t.Fatalf("RefineChapterEnd returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected refinement")
	}
	if refined.End != 124 || refined.Algorithm != ChapterSilenceAlgorithm || refined.Confidence != 0.98 {
		t.Fatalf("unexpected refined segment: %+v", refined)
	}
}

func TestSilenceBoundaryRefinerAcceptsBoundaryAdjacentSilence(t *testing.T) {
	ffmpeg := writeFakeFFmpeg(t, `
echo "[silencedetect @ 0x1] silence_start: 3.5" >&2
echo "[silencedetect @ 0x1] silence_end: 4 | silence_duration: 0.5" >&2
exit 0
`)
	refiner := NewSilenceBoundaryRefiner(DefaultConfig(ffmpeg))
	segment := Segment{Start: 60, End: 120, Confidence: 0.95, Algorithm: ChapterAlgorithm}
	candidate := Candidate{FileID: 1, FilePath: "/tmp/media.mkv", DurationSeconds: 1200}

	refined, ok, err := refiner.RefineChapterEnd(context.Background(), candidate, segment)
	if err != nil {
		t.Fatalf("RefineChapterEnd returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected boundary-adjacent silence to refine")
	}
	if refined.End != 120.5 {
		t.Fatalf("unexpected refined end %.1f", refined.End)
	}
}

func TestSilenceBoundaryRefinerRejectsTooSmallExtension(t *testing.T) {
	ffmpeg := writeFakeFFmpeg(t, `
echo "[silencedetect @ 0x1] silence_start: 3.3" >&2
echo "[silencedetect @ 0x1] silence_end: 3.8 | silence_duration: 0.5" >&2
exit 0
`)
	refiner := NewSilenceBoundaryRefiner(DefaultConfig(ffmpeg))
	segment := Segment{Start: 60, End: 120, Confidence: 0.95, Algorithm: ChapterAlgorithm}
	candidate := Candidate{FileID: 1, FilePath: "/tmp/media.mkv", DurationSeconds: 1200}

	refined, ok, err := refiner.RefineChapterEnd(context.Background(), candidate, segment)
	if err != nil {
		t.Fatalf("RefineChapterEnd returned error: %v", err)
	}
	if ok {
		t.Fatalf("expected no refinement for sub-minimum extension, got %+v", refined)
	}
}

func TestSilenceBoundaryRefinerRejectsUnsafeSilence(t *testing.T) {
	ffmpeg := writeFakeFFmpeg(t, `
echo "[silencedetect @ 0x1] silence_start: 1" >&2
echo "[silencedetect @ 0x1] silence_start: 34" >&2
exit 0
`)
	refiner := NewSilenceBoundaryRefiner(DefaultConfig(ffmpeg))
	segment := Segment{Start: 60, End: 120, Confidence: 0.95, Algorithm: ChapterAlgorithm}
	candidate := Candidate{FileID: 1, FilePath: "/tmp/media.mkv", DurationSeconds: 1200}

	refined, ok, err := refiner.RefineChapterEnd(context.Background(), candidate, segment)
	if err != nil {
		t.Fatalf("RefineChapterEnd returned error: %v", err)
	}
	if ok {
		t.Fatalf("expected no refinement, got %+v", refined)
	}
}

func TestSilenceBoundaryRefinerReturnsErrorOnFFmpegFailure(t *testing.T) {
	ffmpeg := writeFakeFFmpeg(t, `exit 1`)
	refiner := NewSilenceBoundaryRefiner(DefaultConfig(ffmpeg))
	segment := Segment{Start: 60, End: 120, Confidence: 0.95, Algorithm: ChapterAlgorithm}
	candidate := Candidate{FileID: 1, FilePath: "/tmp/media.mkv", DurationSeconds: 1200}

	_, ok, err := refiner.RefineChapterEnd(context.Background(), candidate, segment)
	if err == nil {
		t.Fatal("expected ffmpeg error")
	}
	if ok {
		t.Fatal("failed ffmpeg should not report refinement")
	}
}

func writeFakeFFmpeg(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ffmpeg")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatalf("writing fake ffmpeg: %v", err)
	}
	return path
}
