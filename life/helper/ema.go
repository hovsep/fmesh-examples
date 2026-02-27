package helper

// EMA represents Exponential Moving Average with trend detection
type EMA struct {
	alpha   float64
	value   float64
	epsilon float64 // sensitivity for trend detection
}

// NewEMA creates a new EMA with smoothing factor alpha ∈ (0,1)
// initialValue = starting EMA value
// epsilon = threshold for trend detection
func NewEMA(alpha, initialValue, epsilon float64) *EMA {
	return &EMA{
		alpha:   alpha,
		value:   initialValue,
		epsilon: epsilon,
	}
}

// Update incorporates a new sample and returns the smoothed value
func (e *EMA) Update(sample float64) float64 {
	e.value = e.alpha*sample + (1-e.alpha)*e.value
	return e.value
}

// Value returns the current EMA value
func (e *EMA) Value() float64 {
	return e.value
}

// ClassifyTrend returns +1 for rising, -1 for falling, 0 for balanced
func (e *EMA) ClassifyTrend(current float64) int {
	diff := current - e.value
	switch {
	case diff > e.epsilon:
		return +1
	case diff < -e.epsilon:
		return -1
	default:
		return 0
	}
}
