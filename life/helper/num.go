package helper

import (
	"math/rand"
)

// Clamp restricts value to the range [min, max]
func Clamp(value, minVal, maxVal float64) float64 {
	return max(minVal, min(maxVal, value))
}

// Jitter returns a value randomly jittered by ±percent%
// percent can be decimal, e.g., 0.5 → ±0.5%, 5 → ±5%
func Jitter(value, percent float64) float64 {
	// amplitude = percent of value
	amp := value * percent / 100.0

	// random delta in [-amp, +amp]
	delta := (rand.Float64()*2 - 1) * amp

	return value + delta
}

// Lerp performs linear interpolation between a and b.
// t is typically in [0,1], but is not clamped.
func Lerp(a, b, t float64) float64 {
	return a + (b-a)*t
}
