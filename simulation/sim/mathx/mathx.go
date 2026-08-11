// Package mathx is the small numeric kit a simulation keeps reaching for:
// holding a value inside a range, easing one value toward another, roughening a
// number so it does not look machined.
//
// None of it knows anything about what is being simulated. It lives out here
// rather than in a helper package next to the model precisely so that it cannot
// learn.
package mathx

import (
	"log"
	"math"
	"math/rand"
)

// Number is any type the arithmetic here is willing to average.
type Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// Clamp restricts value to the range [minVal, maxVal].
func Clamp(value, minVal, maxVal float64) float64 {
	return max(minVal, min(maxVal, value))
}

// ClampAndLogAnomaly clamps and says so when it had to.
//
// A value that needed clamping is usually a bug somewhere upstream rather than a
// number at the edge of its range, and silently correcting it hides the thing
// worth knowing.
func ClampAndLogAnomaly(value, minVal, maxVal float64, logger *log.Logger, key string) float64 {
	valueClamped := Clamp(value, minVal, maxVal)
	if valueClamped != value {
		logger.Printf("clamped %s from %f to %f\n", key, value, valueClamped)
	}
	return valueClamped
}

// Jitter returns a value randomly jittered by ±percent%.
// percent can be decimal, e.g. 0.5 → ±0.5%, 5 → ±5%.
func Jitter(value, percent float64) float64 {
	// amplitude = percent of value
	amp := value * percent / 100.0

	// random delta in [-amp, +amp]
	delta := (rand.Float64()*2 - 1) * amp

	return value + delta
}

// Lerp performs linear interpolation between a and b.
// t is typically in [0,1].
func Lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}

// DecayToward moves current toward target by one dt of exponential decay with
// the given half-life (in the same time unit as dt).
//
// Real quantities relax rather than snap: a fright fades over minutes, a full
// stomach empties over hours. Expressing that as a half-life keeps the rate
// independent of the step size, so changing how finely time is sliced does not
// change how fast things settle.
//
// That independence only holds if it is applied once per step. Applying it twice
// halves every half-life, which is the kind of bug that hides until the step
// changes.
func DecayToward(current, target, dt, halfLife float64) float64 {
	if halfLife <= 0 {
		return target
	}
	retained := math.Exp2(-dt / halfLife)
	return target + (current-target)*retained
}

// Mean calculates the arithmetic mean of a slice of numbers.
func Mean[T Number](slice []T) float64 {
	if len(slice) == 0 {
		return 0
	}

	var sum float64
	for _, v := range slice {
		sum += float64(v)
	}

	return sum / float64(len(slice))
}

// Variance is the sample variance (Bessel-corrected), the spread a Welch test
// needs. Fewer than two values have no spread to speak of and give 0.
func Variance[T Number](slice []T) float64 {
	if len(slice) < 2 {
		return 0
	}

	mean := Mean(slice)
	var sum float64
	for _, v := range slice {
		d := float64(v) - mean
		sum += d * d
	}

	return sum / float64(len(slice)-1)
}

// StdDev is the square root of Variance.
func StdDev[T Number](slice []T) float64 {
	return math.Sqrt(Variance(slice))
}

// Welch compares two samples that need not share a variance or a size, and
// returns the t statistic and the Welch-Satterthwaite degrees of freedom.
//
// It answers "are these two sets of readings different by more than their own
// scatter", which is the question a simulation test is really asking: a mesh
// that jitters its regional biases and adds noise to its tone will never produce
// two identical readings, so comparing single values before and after is a coin
// toss dressed as an assertion.
//
// It is worth being blunt about what this does NOT license. A t statistic
// assumes independent samples, and consecutive samples of a simulation
// trajectory are nothing of the kind -- they are the same state one tick apart,
// and a body's temperature this tick is very nearly its temperature last tick.
// Feed it six thousand tick samples and it will report overwhelming significance
// for a difference of no consequence, because the effective sample size is a
// small fraction of the nominal one. Callers must thin their series toward
// independence first (see simtest.Series.Thin) and should require an effect size
// as well as a t, because the effect size is the part that survives the
// assumption being wrong.
//
// Returns t = 0, df = 0 when either sample is too small or neither varies.
func Welch[T Number](a, b []T) (t, df float64) {
	if len(a) < 2 || len(b) < 2 {
		return 0, 0
	}

	na, nb := float64(len(a)), float64(len(b))
	va, vb := Variance(a)/na, Variance(b)/nb

	if va+vb == 0 {
		return 0, 0
	}

	t = (Mean(a) - Mean(b)) / math.Sqrt(va+vb)
	df = (va + vb) * (va + vb) / (va*va/(na-1) + vb*vb/(nb-1))
	return t, df
}

// CohensD is the difference between two means measured in pooled standard
// deviations: how far apart they are in units of how noisy they are.
//
// Unlike Welch it does not grow with the sample size, which is exactly why it is
// the companion an autocorrelated trajectory needs. Two samples that differ by
// one pooled standard deviation are as far apart whether they were sampled a
// hundred times or a million.
//
// Returns 0 when neither sample varies and their means agree, and +Inf when
// they do not vary but differ -- a difference with no scatter at all is as
// significant as a difference gets.
func CohensD[T Number](a, b []T) float64 {
	if len(a) < 2 || len(b) < 2 {
		return 0
	}

	na, nb := float64(len(a)), float64(len(b))
	pooled := math.Sqrt(((na-1)*Variance(a) + (nb-1)*Variance(b)) / (na + nb - 2))
	diff := Mean(a) - Mean(b)

	if pooled == 0 {
		if diff == 0 {
			return 0
		}
		return math.Inf(int(math.Copysign(1, diff)))
	}
	return diff / pooled
}
