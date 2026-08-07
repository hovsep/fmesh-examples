package simtest

import (
	"fmt"
	"math"
	"time"

	"github.com/hovsep/fmesh-examples/simulation/mathx"
)

// Observation is everything a run recorded about one observable.
type Observation struct {
	// Name is the observable this is about.
	Name string

	// Before is the baseline window, sampled before the command was issued.
	Before Series

	// After is the tail of the settling period: where the value ended up.
	After Series

	// During is the whole trajectory from the command to the end of the run, for
	// checks about shape rather than destination.
	During Series

	// Control is the same After window from a run in which no command was
	// issued. It is what tells a value the command moved apart from a value that
	// was going there anyway -- a bladder fills and a fuel reserve drains no
	// matter what anyone types.
	Control Series

	// PerSample is how much simulated time one reading is worth, for the checks
	// that talk about how long something took.
	PerSample time.Duration
}

// Check is one assertion about what a command did to one observable.
//
// Verify returns an error rather than taking a *testing.T so that the vocabulary
// is pure: every check here is tested against synthetic series with known
// answers, which would be impossible if failing meant calling t.Errorf.
type Check interface {
	Describe() string
	Verify(Observation) error
}

// checkFunc is the plumbing every check below is built from.
type checkFunc struct {
	describe string
	verify   func(Observation) error
}

func (c checkFunc) Describe() string           { return c.describe }
func (c checkFunc) Verify(o Observation) error { return c.verify(o) }
func check(describe string, verify func(Observation) error) Check {
	return checkFunc{describe: describe, verify: verify}
}

// The evidence a directional check demands.
//
// Two thresholds rather than one, because neither is sufficient alone. The t
// statistic answers "is this more than the scatter", but it grows with the
// sample count and a trajectory's samples are not independent however much they
// are thinned. Cohen's d answers "by how much, in units of the scatter", and
// does not grow with the sample count at all. Requiring both means a check
// passes when the change is real and big enough to talk about, and fails when it
// is either noise or a rounding error dressed up by ten thousand samples.
const (
	// significantT is the two-sided 1% critical value of the normal
	// distribution, which the t distribution is indistinguishable from at the
	// degrees of freedom a thinned trajectory gives.
	significantT = 2.576

	// meaningfulD is one pooled standard deviation.
	meaningfulD = 1.0

	// thinTo is how many readings a significance test is allowed. See Series.Thin.
	thinTo = 100

	// unmistakable is a proportional change large enough to need no statistics.
	//
	// It exists because an effect size is the wrong tool for an observable that
	// oscillates. A tidal volume swings from nothing to full on every breath, so
	// the scatter within any window is the size of a breath rather than the size
	// of the noise, and a reading that nearly doubles still lands well inside one
	// pooled standard deviation. Cohen's d says "not distinguishable"; a
	// physiologist looking at 5.2 mL becoming 9.3 mL says otherwise, and is right.
	// A change this size is taken at face value.
	unmistakable = 0.25
)

// relativeChange is how far the value moved as a fraction of where it started.
//
// A baseline of zero has no proportion to take, so a move off zero is reported
// as unbounded: going from nothing to something is as large a change as there
// is, and treating it as "no proportion, therefore no change" is how a carbon
// monoxide reading could go from 0 to 1000 ppm without registering.
func relativeChange(before, after float64) float64 {
	if before == 0 {
		if after == 0 {
			return 0
		}
		return math.Copysign(math.Inf(1), after)
	}
	return (after - before) / math.Abs(before)
}

// decisive reports a change so clean it needs no statistics: one where neither
// window varied at all, so there is no scatter for the difference to hide in.
//
// This is the noiseless case, and it was the one the statistics got wrong. A
// command that sets the air to exactly 27 degrees from exactly 26 produces two
// flat series; Welch has no variance to divide by and returns t=0, and Cohen's d
// divides by a pooled deviation of zero and returns infinity. Read literally
// that is "not significant, infinitely large". It is in fact the most certain
// measurement in the suite.
func decisive(d float64) bool { return math.IsInf(d, 0) }

// moved reports the statistics behind a directional check.
func moved(o Observation) (before, after, t, d float64) {
	a, b := o.After.Thin(thinTo), o.Before.Thin(thinTo)
	t, _ = mathx.Welch(a, b)
	return o.Before.Steady(), o.After.Steady(), t, mathx.CohensD(a, b)
}

// Increases asserts the value ended higher than it started, by more than the
// simulation's own noise.
func Increases() Check {
	return check("increase", func(o Observation) error {
		before, after, t, d := moved(o)
		if after > before && (t > significantT && d > meaningfulD ||
			decisive(d) || relativeChange(before, after) > unmistakable) {
			return nil
		}
		return fmt.Errorf("expected an increase, got %.4g -> %.4g (t=%.2f, d=%.2f)",
			before, after, t, d)
	})
}

// Decreases asserts the value ended lower than it started, by more than noise.
func Decreases() Check {
	return check("decrease", func(o Observation) error {
		before, after, t, d := moved(o)
		if after < before && (t < -significantT && d < -meaningfulD ||
			decisive(d) || relativeChange(before, after) < -unmistakable) {
			return nil
		}
		return fmt.Errorf("expected a decrease, got %.4g -> %.4g (t=%.2f, d=%.2f)",
			before, after, t, d)
	})
}

// ChangesBy asserts a proportional change of a given size: -0.5 is "halved",
// +0.3 is "up by a third", each within tolerance (also a fraction).
//
// This is the check to reach for once a direction is established and the
// question becomes whether the magnitude is right. A heart rate that rises by
// two beats and one that rises by ninety both pass Increases.
func ChangesBy(fraction, tolerance float64) Check {
	return check(fmt.Sprintf("change by %+.0f%% (±%.0f%%)", fraction*100, tolerance*100),
		func(o Observation) error {
			before, after := o.Before.Steady(), o.After.Steady()
			if before == 0 {
				return fmt.Errorf("cannot measure a proportional change from a baseline of zero")
			}
			got := after/before - 1
			if math.Abs(got-fraction) <= tolerance {
				return nil
			}
			return fmt.Errorf("expected a change of %+.1f%% (±%.1f%%), got %+.1f%% (%.4g -> %.4g)",
				fraction*100, tolerance*100, got*100, before, after)
		})
}

// StaysExactlyTheSame asserts the value never moved at all, at any point.
//
// Exact, with no tolerance, and that is the point: it is the only way to state
// that a mechanism is genuinely not connected to a command. Everything else
// should be StaysWithin.
func StaysExactlyTheSame() Check {
	return check("stay exactly the same", func(o Observation) error {
		all := append(append(Series{}, o.Before...), o.During...)
		if all.AllSame() {
			return nil
		}
		return fmt.Errorf("expected no change at all, but it moved between %.6g and %.6g",
			all.Min(), all.Max())
	})
}

// StaysWithin asserts the value never strayed further than a fraction of its
// baseline -- allowing whatever the control run drifted by over the same window,
// so a value that was going somewhere anyway is not blamed on the command.
func StaysWithin(fraction float64) Check {
	return check(fmt.Sprintf("stay within %.0f%%", fraction*100), func(o Observation) error {
		baseline := o.Before.Steady()
		scale := math.Abs(baseline)
		if scale == 0 {
			// A baseline of zero has no percentage. Fall back to the span the
			// value showed before the command, so a value that idles at zero is
			// still allowed its own jitter.
			scale = max(o.Before.Span(), 1e-9)
		}

		allowed := fraction * scale
		if drift := math.Abs(o.Control.Steady() - baseline); drift > 0 {
			allowed += drift
		}

		if strayed := math.Abs(o.After.Steady() - baseline); strayed <= allowed {
			return nil
		}
		return fmt.Errorf("expected it to stay put, but it moved %.4g -> %.4g (allowed ±%.3g, control went to %.4g)",
			baseline, o.After.Steady(), allowed, o.Control.Steady())
	})
}

// ReturnsToward asserts the value came back to a remembered baseline.
//
// This is the check the old table had no way to express, and the question this
// whole simulation invites: a pulse that rises when the running starts should
// come back down when it stops, and "it fell a little" is not that.
func ReturnsToward(baseline, tolerance float64) Check {
	return check(fmt.Sprintf("return to %.4g (±%.0f%%)", baseline, tolerance*100),
		func(o Observation) error {
			scale := math.Abs(baseline)
			if scale == 0 {
				scale = 1
			}
			if math.Abs(o.After.Steady()-baseline) <= tolerance*scale {
				return nil
			}
			return fmt.Errorf("expected a return to %.4g (±%.1f%%), but it settled at %.4g",
				baseline, tolerance*100, o.After.Steady())
		})
}

// RisesThenFalls asserts a transient: the value went up, peaked, and came back.
// The peak must clear the surrounding levels by margin, as a fraction, or a
// series that merely wobbled would qualify.
func RisesThenFalls(margin float64) Check {
	return check("rise then fall", func(o Observation) error {
		return transient(o, Up, margin)
	})
}

// FallsThenRises is the dip to RisesThenFalls' spike.
func FallsThenRises(margin float64) Check {
	return check("fall then rise", func(o Observation) error {
		return transient(o, Down, margin)
	})
}

func transient(o Observation, dir Direction, margin float64) error {
	start, end := o.Before.Steady(), o.After.Steady()
	scale := max(math.Abs(start), 1e-9)

	peak, ends := o.During.Max(), max(start, end)
	if dir == Down {
		peak, ends = o.During.Min(), min(start, end)
	}

	if math.Abs(peak-ends) < margin*scale {
		return fmt.Errorf("expected a transient %s of at least %.1f%%, but the extreme was %.4g against levels of %.4g and %.4g",
			dir, margin*100, peak, start, end)
	}
	if math.Abs(end-start) > math.Abs(peak-ends) {
		return fmt.Errorf("expected it to come back, but it went %.4g -> %.4g and stayed (extreme %.4g)",
			start, end, peak)
	}
	return nil
}

// Reaches asserts the value arrived at a particular number, within tolerance as
// a fraction. For the cases where physiology names the answer.
func Reaches(value, tolerance float64) Check {
	return check(fmt.Sprintf("reach %.4g (±%.0f%%)", value, tolerance*100), func(o Observation) error {
		scale := math.Abs(value)
		if scale == 0 {
			scale = 1
		}
		if math.Abs(o.After.Steady()-value) <= tolerance*scale {
			return nil
		}
		return fmt.Errorf("expected it to reach %.4g (±%.1f%%), got %.4g",
			value, tolerance*100, o.After.Steady())
	})
}

// StaysAbove and StaysBelow assert a bound was never crossed at any point in the
// run, which is how a safety margin is stated.

func StaysAbove(limit float64) Check {
	return check(fmt.Sprintf("stay above %.4g", limit), func(o Observation) error {
		if o.During.Len() == 0 || o.During.Min() >= limit {
			return nil
		}
		return fmt.Errorf("expected it to stay above %.4g, but it reached %.4g", limit, o.During.Min())
	})
}

func StaysBelow(limit float64) Check {
	return check(fmt.Sprintf("stay below %.4g", limit), func(o Observation) error {
		if o.During.Len() == 0 || o.During.Max() <= limit {
			return nil
		}
		return fmt.Errorf("expected it to stay below %.4g, but it reached %.4g", limit, o.During.Max())
	})
}

// All requires every one of several checks to pass, so one observable can carry
// a direction and a magnitude and a bound at once.
func All(checks ...Check) Check {
	describe := ""
	for i, c := range checks {
		if i > 0 {
			describe += " and "
		}
		describe += c.Describe()
	}
	return check(describe, func(o Observation) error {
		for _, c := range checks {
			if err := c.Verify(o); err != nil {
				return err
			}
		}
		return nil
	})
}
