package common

// ControlPort is the input port through which a component receives commands
// from outside the simulation. The habitat factors established the convention
// (see env/factor/gas.go) and the body follows it.
const ControlPort = "ctl"

// TimePort is the input port carrying the simulation tick. Components gate their
// per-tick work on it so that an out-of-band command cannot make them emit twice.
const TimePort = "time"

// Scalars carried on the body_state signal that physiology:physiological_state
// broadcasts each tick. Components read the reservoirs from here rather than
// owning copies, so there is one answer to "how hydrated is this body".
const (
	HydrationPct    = "hydration_pct"    // 100 is fully hydrated
	Glycemia        = "glycemia"         // blood glucose, mg/dL
	EnergyKcal      = "energy_kcal"      // usable energy reserve
	CoreTemperature = "core_temperature" // °C
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

// Scalars carried on the per-tick exchange signals (absorption and losses).
// These are amounts gained or lost during the tick, not rates, so integrating
// them is plain addition and no component needs to agree about dt.
const (
	GlucoseKcal = "glucose_kcal"
	WaterMl     = "water_ml"
)

const (
	Sympathetic     Label = "sympathetic"
	Parasympathetic Label = "parasympathetic"
	Noise           Label = "noise"
	Gain            Label = "gain"

	Rate  State = "rate"
	Phase State = "phase"

	Cardiac     System = "cardiac"
	Vascular    System = "vascular"
	Respiratory System = "respiratory"
	GI          System = "gi"

	Balanced Trend = "balanced"
	Rising   Trend = "rising"
	Falling  Trend = "falling"

	Left  Side = "left"
	Right Side = "right"
)
