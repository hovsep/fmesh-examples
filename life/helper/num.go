package helper

import (
	"math/rand"
)

type Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

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
