package intromarkers

import (
	"testing"

	"github.com/Silo-Server/silo-server/internal/models"
)

func TestShouldApplyIntroPatchAllowsSameAlgorithmRangeCorrection(t *testing.T) {
	start := 60.0
	end := 120.0
	source := models.MarkerSourceScanner
	confidence := 0.95
	algorithm := ChapterSilenceAlgorithm
	row := markerRow{
		IntroStart:             &start,
		IntroEnd:               &end,
		IntroMarkersSource:     &source,
		IntroMarkersConfidence: &confidence,
		IntroMarkersAlgorithm:  &algorithm,
	}
	patch := IntroMarkerPatch{
		Start:      60,
		End:        132,
		Source:     models.MarkerSourceScanner,
		Confidence: 0.95,
		Algorithm:  ChapterSilenceAlgorithm,
	}

	if !shouldApplyIntroPatch(row, patch) {
		t.Fatal("expected equal-confidence range correction to apply")
	}
}

func TestShouldApplyIntroPatchRejectsLowerPrioritySource(t *testing.T) {
	start := 60.0
	end := 120.0
	source := models.MarkerSourceManual
	confidence := 1.0
	algorithm := "manual"
	row := markerRow{
		IntroStart:             &start,
		IntroEnd:               &end,
		IntroMarkersSource:     &source,
		IntroMarkersConfidence: &confidence,
		IntroMarkersAlgorithm:  &algorithm,
	}
	patch := IntroMarkerPatch{
		Start:      60,
		End:        132,
		Source:     models.MarkerSourceScanner,
		Confidence: 0.99,
		Algorithm:  ChapterSilenceAlgorithm,
	}

	if shouldApplyIntroPatch(row, patch) {
		t.Fatal("scanner patch should not overwrite manual marker")
	}
}
