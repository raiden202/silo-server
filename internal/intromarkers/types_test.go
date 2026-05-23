package intromarkers

import "testing"

func TestConfigNormalizedDefaultsSilenceNoiseThreshold(t *testing.T) {
	cfg := (Config{}).normalized()
	if cfg.SilenceNoiseThresholdDB == nil || *cfg.SilenceNoiseThresholdDB != -50 {
		t.Fatalf("expected default silence threshold -50, got %v", cfg.SilenceNoiseThresholdDB)
	}
}

func TestConfigNormalizedPreservesExplicitZeroSilenceNoiseThreshold(t *testing.T) {
	cfg := DefaultConfig("ffmpeg")
	cfg.SilenceNoiseThresholdDB = intPtr(0)

	normalized := cfg.normalized()
	if normalized.SilenceNoiseThresholdDB == nil || *normalized.SilenceNoiseThresholdDB != 0 {
		t.Fatalf("expected explicit silence threshold 0, got %v", normalized.SilenceNoiseThresholdDB)
	}
}

func TestConfigNormalizedRejectsPositiveSilenceNoiseThreshold(t *testing.T) {
	cfg := DefaultConfig("ffmpeg")
	cfg.SilenceNoiseThresholdDB = intPtr(5)

	normalized := cfg.normalized()
	if normalized.SilenceNoiseThresholdDB == nil || *normalized.SilenceNoiseThresholdDB != -50 {
		t.Fatalf("expected positive silence threshold to fall back to -50, got %v", normalized.SilenceNoiseThresholdDB)
	}
}
