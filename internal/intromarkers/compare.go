package intromarkers

import (
	"math"
	"math/bits"
	"sort"
)

type fingerprintInput struct {
	Candidate Candidate
	Points    []uint32
}

func CompareFingerprints(inputs []fingerprintInput, cfg Config) map[int]Segment {
	cfg = cfg.normalized()
	best := map[int]Segment{}
	confirmations := map[int]int{}

	for i := 0; i < len(inputs); i++ {
		for j := i + 1; j < len(inputs); j++ {
			left, right := inputs[i], inputs[j]
			if left.Candidate.EpisodeID == "" || left.Candidate.EpisodeID == right.Candidate.EpisodeID {
				continue
			}
			leftSeg, rightSeg, ok := comparePair(left.Points, right.Points, cfg)
			if !ok {
				continue
			}
			leftSeg = adjustSegment(leftSeg, left.Candidate)
			rightSeg = adjustSegment(rightSeg, right.Candidate)
			if validAdjustedSegment(leftSeg) {
				recordBest(best, confirmations, left.Candidate.FileID, leftSeg)
			}
			if validAdjustedSegment(rightSeg) {
				recordBest(best, confirmations, right.Candidate.FileID, rightSeg)
			}
		}
	}

	for fileID, segment := range best {
		confidence := 0.65
		if segment.End-segment.Start >= 30 {
			confidence += 0.10
		}
		if confirmations[fileID] >= 2 {
			confidence += 0.10
		}
		if segment.Start == 0 {
			confidence += 0.05
		}
		if confidence > 0.90 {
			confidence = 0.90
		}
		segment.Confidence = confidence
		segment.Algorithm = ChromaprintAlgorithm
		best[fileID] = segment
	}

	return best
}

func comparePair(left, right []uint32, cfg Config) (Segment, Segment, bool) {
	if len(left) == 0 || len(right) == 0 {
		return Segment{}, Segment{}, false
	}

	shifts := candidateShifts(left, right)
	var bestLeft, bestRight Segment
	bestDuration := 0.0
	for _, shift := range shifts {
		leftSeg, rightSeg, ok := comparePairAtShift(left, right, cfg, shift)
		if !ok {
			continue
		}
		if duration := leftSeg.End - leftSeg.Start; duration > bestDuration {
			bestLeft = leftSeg
			bestRight = rightSeg
			bestDuration = duration
		}
	}
	if bestDuration == 0 {
		return Segment{}, Segment{}, false
	}
	return bestLeft, bestRight, true
}

func comparePairAtShift(left, right []uint32, cfg Config, shift int) (Segment, Segment, bool) {
	type pair struct {
		left  int
		right int
	}
	var matches []pair
	tolerance := candidateShiftSamplePoints()
	for i, lp := range left {
		center := i + shift
		from := max(0, center-tolerance)
		to := min(len(right)-1, center+tolerance)
		if to < from {
			continue
		}
		for j := from; j <= to; j++ {
			if bits.OnesCount32(lp^right[j]) <= 6 {
				matches = append(matches, pair{left: i, right: j})
				break
			}
		}
	}
	if len(matches) == 0 {
		return Segment{}, Segment{}, false
	}

	maxGapPoints := int(math.Ceil(3.5 / DefaultPointHopSeconds))
	bestStart := 0
	bestEnd := 0
	bestRunStart := 0
	runStart := 0
	for i := 1; i < len(matches); i++ {
		leftGap := matches[i].left - matches[i-1].left
		rightGap := matches[i].right - matches[i-1].right
		if leftGap <= maxGapPoints && rightGap >= -candidateShiftSamplePoints() && rightGap <= maxGapPoints {
			continue
		}
		if matches[i-1].left-matches[runStart].left > bestEnd-bestStart {
			bestStart = matches[runStart].left
			bestEnd = matches[i-1].left
			bestRunStart = runStart
		}
		runStart = i
	}
	if matches[len(matches)-1].left-matches[runStart].left > bestEnd-bestStart {
		bestStart = matches[runStart].left
		bestEnd = matches[len(matches)-1].left
		bestRunStart = runStart
	}

	start := float64(bestStart) * DefaultPointHopSeconds
	end := float64(bestEnd+1) * DefaultPointHopSeconds
	duration := end - start
	if duration < float64(cfg.MinimumIntroDurationSeconds) || duration > float64(cfg.MaximumIntroDurationSeconds) {
		return Segment{}, Segment{}, false
	}

	rightShift := matches[bestRunStart].right - matches[bestRunStart].left
	rightStart := float64(max(0, bestStart+rightShift)) * DefaultPointHopSeconds
	rightEnd := rightStart + duration
	return Segment{Start: start, End: end}, Segment{Start: rightStart, End: rightEnd}, true
}

func candidateShifts(left, right []uint32) []int {
	step := candidateShiftSamplePoints()
	counts := map[int]int{}
	for i := 0; i < len(left); i += step {
		for j := 0; j < len(right); j += step {
			if bits.OnesCount32(left[i]^right[j]) <= 6 {
				counts[j-i]++
			}
		}
	}

	type candidate struct {
		shift int
		count int
	}
	candidates := []candidate{{shift: 0, count: counts[0]}}
	for shift, count := range counts {
		if shift == 0 || count < 3 {
			continue
		}
		candidates = append(candidates, candidate{shift: shift, count: count})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].count != candidates[j].count {
			return candidates[i].count > candidates[j].count
		}
		if absInt(candidates[i].shift) != absInt(candidates[j].shift) {
			return absInt(candidates[i].shift) < absInt(candidates[j].shift)
		}
		return candidates[i].shift < candidates[j].shift
	})

	limit := min(8, len(candidates))
	shifts := make([]int, 0, limit)
	seen := map[int]struct{}{}
	for _, candidate := range candidates {
		if len(shifts) >= limit {
			break
		}
		if _, ok := seen[candidate.shift]; ok {
			continue
		}
		seen[candidate.shift] = struct{}{}
		shifts = append(shifts, candidate.shift)
	}
	return shifts
}

func candidateShiftSamplePoints() int {
	return max(1, int(math.Round(1/DefaultPointHopSeconds)))
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func adjustSegment(segment Segment, candidate Candidate) Segment {
	if segment.Start <= 5 {
		segment.Start = 0
	}
	for _, chapter := range candidate.Chapters {
		segment.Start = snapBoundary(segment.Start, chapter.StartSeconds)
		segment.End = snapBoundary(segment.End, chapter.StartSeconds)
		segment.End = snapBoundary(segment.End, chapter.EndSeconds)
	}
	if segment.Start < 0 {
		segment.Start = 0
	}
	if candidate.DurationSeconds > 0 && segment.End > candidate.DurationSeconds {
		segment.End = candidate.DurationSeconds
	}
	return segment
}

func snapBoundary(value, boundary float64) float64 {
	delta := boundary - value
	if delta >= -5 && delta <= 2 {
		return boundary
	}
	return value
}

func validAdjustedSegment(segment Segment) bool {
	duration := segment.End - segment.Start
	return segment.Start >= 0 && segment.End > segment.Start && duration >= 10 && duration <= 180
}

func recordBest(best map[int]Segment, confirmations map[int]int, fileID int, segment Segment) {
	confirmations[fileID]++
	if current, ok := best[fileID]; !ok || segment.End-segment.Start > current.End-current.Start {
		best[fileID] = segment
	}
}
