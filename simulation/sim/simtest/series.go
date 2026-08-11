package simtest

import (
	"math"
	"time"

	"github.com/hovsep/fmesh-examples/simulation/sim/mathx"
)

// Series is one observable's readings over a stretch of a run, oldest first.
//
// Every statistic here is descriptive rather than inferential: it says what the
// numbers did, and leaves what that means to a Check.
type Series []float64

// Direction is which way a series is expected to go.
type Direction int

const (
	Up Direction = iota
	Down
)

func (d Direction) String() string {
	if d == Up {
		return "up"
	}
	return "down"
}

// Len, Mean, StdDev, Min, Max, First and Last are the obvious readings. Each
// gives 0 on an empty series rather than panicking, because a series can be
// empty for a legitimate reason -- an observable the mesh never published -- and
// the runner reports that once, clearly, instead of everything downstream
// panicking about it.

func (s Series) Len() int        { return len(s) }
func (s Series) Mean() float64   { return mathx.Mean(s) }
func (s Series) StdDev() float64 { return mathx.StdDev(s) }

func (s Series) First() float64 {
	if len(s) == 0 {
		return 0
	}
	return s[0]
}

func (s Series) Last() float64 {
	if len(s) == 0 {
		return 0
	}
	return s[len(s)-1]
}

func (s Series) Min() float64 {
	if len(s) == 0 {
		return 0
	}
	lowest := s[0]
	for _, v := range s[1:] {
		lowest = min(lowest, v)
	}
	return lowest
}

func (s Series) Max() float64 {
	if len(s) == 0 {
		return 0
	}
	highest := s[0]
	for _, v := range s[1:] {
		highest = max(highest, v)
	}
	return highest
}

// Steady is the mean of the second half: what the value settled at, once
// whatever transient the window opened with has passed.
//
// It is the reading almost every assertion should use. The last sample alone is
// one draw from a noisy process, and the mean of the whole window is dragged by
// the approach rather than describing the destination.
func (s Series) Steady() float64 {
	if len(s) == 0 {
		return 0
	}
	return mathx.Mean(s[len(s)/2:])
}

// Span is how far the series travelled from lowest to highest.
func (s Series) Span() float64 { return s.Max() - s.Min() }

// Overshoot is how far past a target the series went, in the direction it was
// heading, and 0 if it never passed it. It is what tells a control loop that
// settles from one that rings.
//
// Which way it was heading is decided by its larger excursion from where it
// started, rather than by comparing the start against the target. A series that
// begins exactly at the target -- a body at rest, disturbed -- has no side to be
// on, and reading its direction off that comparison reports a spike of nothing.
func (s Series) Overshoot(target float64) float64 {
	if len(s) == 0 {
		return 0
	}
	if s.Max()-s.First() >= s.First()-s.Min() {
		return max(s.Max()-target, 0)
	}
	return max(target-s.Min(), 0)
}

// SettlingTime is how long the series took to reach a band around target and
// stay there, given how much simulated time one sample is worth.
//
// The tolerance is a fraction of the target, so 0.05 is "within five percent".
// Returns -1 if it never settled, which a caller must tell apart from zero.
func (s Series) SettlingTime(target, tolerance float64, perSample time.Duration) time.Duration {
	band := math.Abs(target * tolerance)

	// Walk back from the end: the answer is the first sample after the last one
	// that was still outside the band.
	settledAt := 0
	for i := len(s) - 1; i >= 0; i-- {
		if math.Abs(s[i]-target) > band {
			settledAt = i + 1
			break
		}
	}
	if settledAt >= len(s) {
		return -1
	}
	return time.Duration(settledAt) * perSample
}

// DirectionChanges counts how many times the series turned around, ignoring
// stretches where it did not move at all.
//
// One turn is a rise-then-fall, which is the shape of a transient. Many turns is
// either noise or an oscillation, and which one it is depends on the amplitude,
// so this is reported rather than judged.
func (s Series) DirectionChanges() int {
	changes, previous := 0, 0
	for i := 1; i < len(s); i++ {
		direction := 0
		switch {
		case s[i] > s[i-1]:
			direction = 1
		case s[i] < s[i-1]:
			direction = -1
		default:
			continue
		}
		if previous != 0 && direction != previous {
			changes++
		}
		previous = direction
	}
	return changes
}

// IsMonotonic reports whether the series only ever moved the one way, flat
// stretches allowed.
func (s Series) IsMonotonic(dir Direction) bool {
	for i := 1; i < len(s); i++ {
		if dir == Up && s[i] < s[i-1] {
			return false
		}
		if dir == Down && s[i] > s[i-1] {
			return false
		}
	}
	return true
}

// AllSame reports whether every reading is bitwise identical to the first.
//
// Deliberately exact. "Did not change" is the one assertion that should not have
// a tolerance: a parameter frozen at precisely its initial value is a mechanism
// that never ran, and rounding that to "close enough" is how a stroke volume
// sits at 70.00 mL through a sprint without anything noticing.
func (s Series) AllSame() bool {
	for _, v := range s {
		if v != s.First() {
			return false
		}
	}
	return true
}

// Thin reduces the series to at most n readings, evenly spaced.
//
// This is what makes a significance test on a trajectory mean anything. Samples
// one tick apart are very nearly the same measurement, so a t statistic over all
// of them is inflated by however many times the same information was counted.
// Spreading n readings across the whole window buys samples that are at least
// closer to independent. See mathx.Welch.
func (s Series) Thin(n int) Series {
	if n <= 0 || len(s) <= n {
		return s
	}

	thinned := make(Series, 0, n)
	for i := range n {
		thinned = append(thinned, s[i*len(s)/n])
	}
	return thinned
}
