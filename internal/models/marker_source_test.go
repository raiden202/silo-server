package models

import "testing"

func TestMarkerSourcePriority(t *testing.T) {
	if MarkerSourcePriority(MarkerSourceOnline) <= MarkerSourcePriority(MarkerSourceScanner) {
		t.Fatal("online marker sources must outrank scanner markers")
	}
	if MarkerSourcePriority(MarkerSourceScanner) >= MarkerSourcePriority(MarkerSourceOnline) {
		t.Fatal("scanner markers must not outrank online marker sources")
	}
	if MarkerSourcePriority(MarkerSourceManual) <= MarkerSourcePriority(MarkerSourceOnline) {
		t.Fatal("manual markers must remain highest priority")
	}
}
