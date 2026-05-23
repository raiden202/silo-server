package userstore

// ProgressThresholds bundles the watched and min-resume threshold percentages.
// Zero values mean "use defaults" (90% watched, 5% min-resume).
type ProgressThresholds struct {
	WatchedPct   int // mark completed above this % (default 90)
	MinResumePct int // discard progress below this % (default 5)
}

// WatchedFraction converts a watched-threshold percentage (e.g. 90) to a
// fraction (0.9). If pct <= 0, returns the default of 0.9 (90%).
func WatchedFraction(pct int) float64 {
	if pct <= 0 {
		pct = 90
	}
	return float64(pct) / 100.0
}

// MinResumeFraction converts a min-resume percentage (e.g. 5) to a fraction
// (0.05). If pct <= 0, returns the default of 0.05 (5%).
func MinResumeFraction(pct int) float64 {
	if pct <= 0 {
		pct = 5
	}
	return float64(pct) / 100.0
}
