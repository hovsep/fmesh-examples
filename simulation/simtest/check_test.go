package simtest

import (
	"math"
	"math/rand"
	"testing"

	"github.com/hovsep/fmesh-examples/simulation/mathx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noisy builds a series of n readings around mean, with the given scatter, from
// a fixed seed so every run of this file judges the same numbers.
func noisy(n int, mean, scatter float64, seed int64) Series {
	rng := rand.New(rand.NewSource(seed))
	s := make(Series, n)
	for i := range s {
		s[i] = mean + rng.NormFloat64()*scatter
	}
	return s
}

// ramp builds a series that walks from one value to another.
func ramp(n int, from, to float64) Series {
	s := make(Series, n)
	for i := range s {
		s[i] = from + (to-from)*float64(i)/float64(n-1)
	}
	return s
}

func flat(n int, value float64) Series {
	s := make(Series, n)
	for i := range s {
		s[i] = value
	}
	return s
}

func obs(before, after Series) Observation {
	return Observation{
		Name: "x", Before: before, After: after, During: after,
		Control: after,
	}
}

func TestSeriesStatistics(t *testing.T) {
	s := Series{1, 2, 3, 4, 100}

	assert.Equal(t, 1.0, s.First())
	assert.Equal(t, 100.0, s.Last())
	assert.Equal(t, 1.0, s.Min())
	assert.Equal(t, 100.0, s.Max())
	assert.Equal(t, 99.0, s.Span())
	assert.InDelta(t, 22.0, s.Mean(), 1e-9)

	// Steady is the second half, which is what makes it useful: the mean of the
	// whole series is dragged by wherever it started.
	assert.InDelta(t, mathx.Mean([]float64{3, 4, 100}), s.Steady(), 1e-9)

	assert.True(t, s.IsMonotonic(Up))
	assert.False(t, s.IsMonotonic(Down))
	assert.Equal(t, 0, s.DirectionChanges())

	assert.Equal(t, 2, Series{1, 5, 1, 5}.DirectionChanges())
	assert.True(t, flat(10, 7).AllSame())
	assert.False(t, Series{7, 7, 7.0000001}.AllSame())

	// Every statistic tolerates an empty series rather than panicking, because
	// an unpublished observable produces one.
	var empty Series
	assert.Equal(t, 0.0, empty.Mean())
	assert.Equal(t, 0.0, empty.Min())
	assert.Equal(t, 0.0, empty.Steady())
}

func TestSeriesOvershootAndSettling(t *testing.T) {
	// Rises to 130, comes back to 100.
	spike := append(ramp(50, 100, 130), ramp(50, 130, 100)...)
	assert.InDelta(t, 30, spike.Overshoot(100), 1e-9)
	assert.Equal(t, 0.0, ramp(50, 100, 120).Overshoot(130), "never reached the target")

	// Settles into ±5% of 100 halfway through and stays.
	settling := append(ramp(50, 200, 100), flat(50, 100)...)
	assert.Positive(t, settling.SettlingTime(100, 0.05, 1))
	assert.EqualValues(t, -1, ramp(50, 100, 200).SettlingTime(100, 0.01, 1),
		"a series that never settles must be told apart from one that settled at once")
}

func TestThinningIsWhatMakesSignificanceHonest(t *testing.T) {
	// A trajectory's samples are not independent: this is a slow drift, sampled
	// densely, with no noise at all beyond the drift itself. The two halves
	// differ by a whisker.
	before := ramp(5000, 100, 100.2)
	after := ramp(5000, 100.2, 100.4)

	full, _ := mathx.Welch(after, before)
	thinned, _ := mathx.Welch(after.Thin(thinTo), before.Thin(thinTo))

	assert.Greater(t, math.Abs(full), math.Abs(thinned),
		"an unthinned trajectory inflates its own significance; that is the whole reason Thin exists")

	// Cohen's d is the part that does not care how often you looked.
	assert.InDelta(t,
		mathx.CohensD(after, before),
		mathx.CohensD(after.Thin(thinTo), before.Thin(thinTo)),
		0.15, "effect size should barely move when the sample count changes")
}

func TestDirectionalChecks(t *testing.T) {
	quiet := noisy(400, 100, 1, 1)
	higher := noisy(400, 110, 1, 2)
	lower := noisy(400, 90, 1, 3)
	jitterOnly := noisy(400, 100.05, 1, 4)

	assert.NoError(t, Increases().Verify(obs(quiet, higher)))
	assert.Error(t, Increases().Verify(obs(quiet, lower)))

	assert.NoError(t, Decreases().Verify(obs(quiet, lower)))
	assert.Error(t, Decreases().Verify(obs(quiet, higher)))

	// The point of the significance threshold: a change smaller than the
	// scatter is not a change, however consistently it points one way.
	assert.Error(t, Increases().Verify(obs(quiet, jitterOnly)),
		"a shift well inside the noise must not read as an increase")

	// And a flat series that genuinely did not move.
	assert.Error(t, Increases().Verify(obs(flat(400, 100), flat(400, 100))))
}

func TestChangesBy(t *testing.T) {
	before, after := flat(100, 70), flat(100, 94.5)

	assert.NoError(t, ChangesBy(0.35, 0.05).Verify(obs(before, after)))
	assert.Error(t, ChangesBy(0.10, 0.05).Verify(obs(before, after)),
		"35% is not 10%")
	assert.NoError(t, ChangesBy(-0.5, 0.05).Verify(obs(flat(100, 70), flat(100, 35))))
	assert.Error(t, ChangesBy(0.3, 0.05).Verify(obs(flat(100, 0), flat(100, 5))),
		"a proportional change from zero is not a number")
}

func TestStaysChecks(t *testing.T) {
	steady := flat(200, 5.0)

	o := Observation{Name: "x", Before: steady, After: steady, During: steady, Control: steady}
	assert.NoError(t, StaysExactlyTheSame().Verify(o))
	assert.NoError(t, StaysWithin(0.01).Verify(o))

	// The exactness is deliberate: one part in a million is still a change.
	drifted := append(flat(100, 5.0), flat(100, 5.000001)...)
	assert.Error(t, StaysExactlyTheSame().Verify(
		Observation{Name: "x", Before: steady, After: drifted, During: drifted, Control: steady}),
		"StaysExactlyTheSame must have no tolerance at all")
	assert.NoError(t, StaysWithin(0.01).Verify(
		Observation{Name: "x", Before: steady, After: drifted, During: drifted, Control: steady}))

	// A 20% move is not staying put...
	moved := flat(200, 6.0)
	assert.Error(t, StaysWithin(0.02).Verify(
		Observation{Name: "x", Before: steady, After: moved, During: moved, Control: steady}))

	// ...unless the control run went there too, in which case the command did
	// not do it. This is what keeps the backstop from blaming every command for
	// a bladder that fills on its own.
	assert.NoError(t, StaysWithin(0.02).Verify(
		Observation{Name: "x", Before: steady, After: moved, During: moved, Control: moved}))
}

func TestReturnsToward(t *testing.T) {
	assert.NoError(t, ReturnsToward(64, 0.1).Verify(obs(flat(100, 64), flat(100, 67))))

	// The failure the old suite could not express: a pulse that barely came down.
	assert.Error(t, ReturnsToward(64, 0.1).Verify(obs(flat(100, 64), flat(100, 150))),
		"155 -> 154 is not a return to rest")
}

func TestTransientChecks(t *testing.T) {
	spike := append(ramp(50, 100, 140), ramp(50, 140, 101)...)
	o := Observation{Name: "x", Before: flat(50, 100), After: flat(50, 101), During: spike}
	assert.NoError(t, RisesThenFalls(0.1).Verify(o))

	// A step that went up and stayed up is not a transient.
	step := ramp(100, 100, 140)
	assert.Error(t, RisesThenFalls(0.1).Verify(
		Observation{Name: "x", Before: flat(50, 100), After: flat(50, 140), During: step}))

	dip := append(ramp(50, 100, 60), ramp(50, 60, 99)...)
	assert.NoError(t, FallsThenRises(0.1).Verify(
		Observation{Name: "x", Before: flat(50, 100), After: flat(50, 99), During: dip}))
}

func TestBoundsAndComposition(t *testing.T) {
	during := append(ramp(50, 100, 130), ramp(50, 130, 100)...)
	o := Observation{Name: "x", Before: flat(50, 100), After: flat(50, 100), During: during}

	assert.NoError(t, StaysBelow(140).Verify(o))
	assert.Error(t, StaysBelow(120).Verify(o))
	assert.NoError(t, StaysAbove(90).Verify(o))
	assert.Error(t, StaysAbove(110).Verify(o))

	assert.NoError(t, Reaches(100, 0.05).Verify(o))

	both := All(StaysBelow(140), StaysAbove(90))
	assert.NoError(t, both.Verify(o))
	assert.Contains(t, both.Describe(), "and")
	assert.Error(t, All(StaysBelow(140), StaysAbove(110)).Verify(o))
}

func TestEveryCheckDescribesItself(t *testing.T) {
	// A failure message is only useful if it says what was wanted.
	for _, c := range []Check{
		Increases(), Decreases(), ChangesBy(0.3, 0.1), StaysExactlyTheSame(),
		StaysWithin(0.02), ReturnsToward(64, 0.1), RisesThenFalls(0.1),
		FallsThenRises(0.1), Reaches(37, 0.01), StaysAbove(0), StaysBelow(1),
	} {
		require.NotEmpty(t, c.Describe())
	}
}

func TestAnUnmistakableChangeNeedsNoStatistics(t *testing.T) {
	// An oscillating observable: a tidal volume swings from nothing to full on
	// every breath, so the scatter inside any window is the size of a breath.
	// Breathing twice as deeply is unmistakable, and Cohen's d still cannot see
	// it -- the pooled deviation is the waveform, not the noise.
	breathing := func(peak float64) Series {
		s := make(Series, 400)
		for i := range s {
			s[i] = peak * math.Abs(math.Sin(float64(i)/8))
		}
		return s
	}

	// The ratio the real case showed: a tidal volume that rose by about a third.
	shallow, deep := breathing(5.2), breathing(7.0)

	require.Less(t, math.Abs(mathx.CohensD(deep.Thin(thinTo), shallow.Thin(thinTo))), meaningfulD,
		"this test is pointless unless the effect size really is too small to see")

	assert.NoError(t, Increases().Verify(obs(shallow, deep)),
		"a near-doubling must register however noisy the waveform is")
	assert.NoError(t, Decreases().Verify(obs(deep, shallow)))

	// The escape hatch must not swallow small changes, which is the whole reason
	// the statistics are there.
	assert.Error(t, Increases().Verify(obs(breathing(5.2), breathing(5.4))))
}

func TestANoiselessChangeIsTheMostCertainOfAll(t *testing.T) {
	// Two flat series. Welch has no variance to divide by and reports t=0;
	// Cohen's d divides by a pooled deviation of zero and reports infinity. Read
	// literally that is "not significant, infinitely large", and a first cut of
	// these checks rejected it -- so a carbon monoxide reading going from 0 to
	// 1000 ppm did not count as an increase.
	assert.NoError(t, Increases().Verify(obs(flat(200, 26), flat(200, 27))))
	assert.NoError(t, Increases().Verify(obs(flat(200, 0), flat(200, 1000))))
	assert.NoError(t, Decreases().Verify(obs(flat(200, 1000), flat(200, 0))))

	// The direction still has to be right.
	assert.Error(t, Increases().Verify(obs(flat(200, 27), flat(200, 26))))

	// And a series that genuinely did not move is still not a change.
	assert.Error(t, Increases().Verify(obs(flat(200, 26), flat(200, 26))))
}
