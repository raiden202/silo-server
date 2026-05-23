package recipes

import (
	"testing"
	"time"
)

func TestRealClockReturnsCurrentTime(t *testing.T) {
	c := RealClock{}
	now := c.Now()
	if time.Since(now) > time.Second {
		t.Fatalf("RealClock.Now drifted: %v", now)
	}
}

func TestFixedClockReturnsConfiguredTime(t *testing.T) {
	want := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	c := FixedClock(want)
	if !c.Now().Equal(want) {
		t.Fatalf("got %v want %v", c.Now(), want)
	}
}
