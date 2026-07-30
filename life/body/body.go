// Package body is the vocabulary a body uses about its own condition: the
// reservoirs it defends, what it gains and loses each tick, and the words it has
// for how it feels.
//
// It is a leaf on purpose, the way bloodstream is. Every one of these names is
// read by several packages that have no business importing each other -- an
// organ that reports water loss, a physiology component that integrates it, a UI
// that draws it -- so the shared words have to live somewhere none of them owns.
package body

// Scalars carried on the body_state signal that physiology:physiological_state
// broadcasts each tick. Components read the reservoirs from here rather than
// owning copies, so there is one answer to "how hydrated is this body".
const (
	HydrationPct    = "hydration_pct"    // 100 is fully hydrated
	Glycemia        = "glycemia"         // blood glucose, mg/dL
	EnergyKcal      = "energy_kcal"      // usable energy reserve
	CoreTemperature = "core_temperature" // °C
)

// Scalars carried on the per-tick exchange signals (absorption and losses).
// These are amounts gained or lost during the tick, not rates, so integrating
// them is plain addition and no component needs to agree about dt.
const (
	GlucoseKcal = "glucose_kcal"
	WaterMl     = "water_ml"
)

// Feelings are the words the body uses about its own condition. They are also
// the scalar names on the feelings signal, so what a UI displays is exactly what
// the body reported, with no translation table in between.
const (
	FeelingHungry         = "hungry"
	FeelingThirsty        = "thirsty"
	FeelingTired          = "tired"
	FeelingExhausted      = "exhausted"
	FeelingBreathless     = "breathless"
	FeelingNeedToUrinate  = "need_to_urinate"
	FeelingNeedToDefecate = "need_to_defecate"
	FeelingHeadache       = "headache"
	FeelingFeverish       = "feverish"
	FeelingAnxious        = "anxious"
	FeelingHappy          = "happy"
	FeelingContent        = "content"
)

// Feelings lists every feeling, most alarming first. A UI showing them sorted by
// intensity uses this order to break ties, which stops the list reshuffling
// between frames when several feelings are equally weak.
var Feelings = []string{
	FeelingExhausted,
	FeelingBreathless,
	FeelingHeadache,
	FeelingFeverish,
	FeelingThirsty,
	FeelingHungry,
	FeelingNeedToUrinate,
	FeelingNeedToDefecate,
	FeelingAnxious,
	FeelingTired,
	FeelingHappy,
	FeelingContent,
}

// FeelingLabels are the human-readable forms, for display.
var FeelingLabels = map[string]string{
	FeelingExhausted:      "Exhausted",
	FeelingBreathless:     "Breathless",
	FeelingHeadache:       "Headache",
	FeelingFeverish:       "Feverish",
	FeelingThirsty:        "Thirsty",
	FeelingHungry:         "Hungry",
	FeelingNeedToUrinate:  "Needs to urinate",
	FeelingNeedToDefecate: "Needs the toilet",
	FeelingAnxious:        "Anxious",
	FeelingTired:          "Tired",
	FeelingHappy:          "Happy",
	FeelingContent:        "Content",
}

// Which way a level is moving. A reading on its own says less than its
// direction: 90 mg/dL falling is a different situation from 90 mg/dL rising.
const (
	Balanced = "balanced"
	Rising   = "rising"
	Falling  = "falling"
)

// EMA is an exponential moving average with trend detection: a smoothed value,
// and a verdict on which way the raw readings are going relative to it.
type EMA struct {
	alpha   float64
	value   float64
	epsilon float64 // how far from the average counts as a real move
}

// NewEMA creates an EMA with smoothing factor alpha in (0,1), a starting value,
// and the threshold beyond which a difference counts as a trend.
func NewEMA(alpha, initialValue, epsilon float64) *EMA {
	return &EMA{alpha: alpha, value: initialValue, epsilon: epsilon}
}

// Update incorporates a new sample and returns the smoothed value.
func (e *EMA) Update(sample float64) float64 {
	e.value = e.alpha*sample + (1-e.alpha)*e.value
	return e.value
}

// Value returns the current smoothed value.
func (e *EMA) Value() float64 { return e.value }

// ClassifyTrend reports where a reading sits relative to the smoothed average.
func (e *EMA) ClassifyTrend(current float64) string {
	switch diff := current - e.value; {
	case diff > e.epsilon:
		return Rising
	case diff < -e.epsilon:
		return Falling
	default:
		return Balanced
	}
}

// TrendCode encodes a trend as a number so it can travel over the numeric
// telemetry wire: rising +1, falling -1, balanced 0.
func TrendCode(t string) float64 {
	switch t {
	case Rising:
		return 1
	case Falling:
		return -1
	default:
		return 0
	}
}
