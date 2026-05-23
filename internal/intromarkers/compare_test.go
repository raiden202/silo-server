package intromarkers

import "testing"

func TestCompareFingerprintsFindsSharedRange(t *testing.T) {
	left := make([]uint32, 400)
	right := make([]uint32, 400)
	for i := range left {
		left[i] = uint32(i + 1000)
		right[i] = uint32(i + 5000)
	}
	for i := 40; i < 300; i++ {
		left[i] = uint32(i)
		right[i] = uint32(i)
	}

	segments := CompareFingerprints([]fingerprintInput{
		{Candidate: Candidate{FileID: 1, EpisodeID: "ep1", DurationSeconds: 1200}, Points: left},
		{Candidate: Candidate{FileID: 2, EpisodeID: "ep2", DurationSeconds: 1200}, Points: right},
	}, DefaultConfig("ffmpeg"))

	if len(segments) != 2 {
		t.Fatalf("expected two file segments, got %d", len(segments))
	}
	if got := segments[1].End - segments[1].Start; got < 30 {
		t.Fatalf("expected at least 30s segment, got %.3f", got)
	}
	if segments[1].Algorithm != ChromaprintAlgorithm {
		t.Fatalf("unexpected algorithm %q", segments[1].Algorithm)
	}
}

func TestCompareFingerprintsSkipsSameEpisodePairs(t *testing.T) {
	points := make([]uint32, 400)
	for i := range points {
		points[i] = uint32(i)
	}
	segments := CompareFingerprints([]fingerprintInput{
		{Candidate: Candidate{FileID: 1, EpisodeID: "ep1", DurationSeconds: 1200}, Points: points},
		{Candidate: Candidate{FileID: 2, EpisodeID: "ep1", DurationSeconds: 1200}, Points: points},
	}, DefaultConfig("ffmpeg"))
	if len(segments) != 0 {
		t.Fatalf("same-episode fingerprints must not produce segments: %#v", segments)
	}
}

func TestCompareFingerprintsFindsSharedRangeWithOffset(t *testing.T) {
	left := make([]uint32, 700)
	right := make([]uint32, 700)
	for i := range left {
		left[i] = 0xAAAAAAAA ^ uint32(i)
		right[i] = 0x55555555 ^ uint32(i*3)
	}
	for i := 40; i < 320; i++ {
		point := uint32((i * 17) + 12345)
		left[i] = point
		right[i+160] = point
	}

	segments := CompareFingerprints([]fingerprintInput{
		{Candidate: Candidate{FileID: 1, EpisodeID: "ep1", DurationSeconds: 1200}, Points: left},
		{Candidate: Candidate{FileID: 2, EpisodeID: "ep2", DurationSeconds: 1200}, Points: right},
	}, DefaultConfig("ffmpeg"))

	if len(segments) != 2 {
		t.Fatalf("expected two file segments, got %d", len(segments))
	}
	if segments[1].Start != 0 {
		t.Fatalf("unexpected left start %.3f", segments[1].Start)
	}
	if segments[2].Start < 23 || segments[2].Start > 26 {
		t.Fatalf("unexpected right start %.3f", segments[2].Start)
	}
	if got := segments[1].End - segments[1].Start; got < 30 {
		t.Fatalf("expected at least 30s segment, got %.3f", got)
	}
}

func TestComparePairAtShiftBreaksRunsOnBackwardRightJump(t *testing.T) {
	left := make([]uint32, 160)
	right := make([]uint32, 160)
	for i := range left {
		left[i] = 0xAAAAAAAA ^ uint32(i*17)
		right[i] = 0x55555555 ^ uint32(i*31)
	}
	left[0] = 0x11111111
	right[30] = 0x11111111
	for i := 1; i < 120; i++ {
		point := 0x22220000 + uint32(i)
		left[i] = point
		right[i-1] = point
	}

	cfg := DefaultConfig("ffmpeg")
	cfg.MinimumIntroDurationSeconds = 1
	leftSegment, rightSegment, ok := comparePairAtShift(left, right, cfg, 0)
	if !ok {
		t.Fatal("expected monotonic run after backward jump")
	}
	if leftSegment.Start == 0 {
		t.Fatalf("backward right jump should start a new run, got left segment %+v right segment %+v", leftSegment, rightSegment)
	}
}

func TestComparePairAtShiftAllowsSmallBackwardJitter(t *testing.T) {
	left := make([]uint32, 180)
	right := make([]uint32, 180)
	for i := range left {
		left[i] = 0xAAAAAAAA ^ uint32(i*17)
		right[i] = 0x55555555 ^ uint32(i*31)
	}
	for i := 10; i < 150; i++ {
		point := 0x33330000 + uint32(i)
		left[i] = point
		right[i] = point
	}
	right[70] = 0x55555555
	right[65] = left[71]

	cfg := DefaultConfig("ffmpeg")
	cfg.MinimumIntroDurationSeconds = 1
	leftSegment, _, ok := comparePairAtShift(left, right, cfg, 0)
	if !ok {
		t.Fatal("expected small backward jitter to remain in the same run")
	}
	if got := leftSegment.End - leftSegment.Start; got < 15 {
		t.Fatalf("expected jitter-tolerant run, got duration %.3f", got)
	}
}
