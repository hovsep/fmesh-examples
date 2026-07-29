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
