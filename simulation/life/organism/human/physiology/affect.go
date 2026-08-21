package physiology

import (
	"context"
	"fmt"

	"github.com/hovsep/fmesh-examples/simulation/life/bloodstream"
	"github.com/hovsep/fmesh-examples/simulation/life/body"
	"github.com/hovsep/fmesh-examples/simulation/sim"
	"github.com/hovsep/fmesh-examples/simulation/sim/mathx"
	"github.com/hovsep/fmesh-examples/simulation/sim/simtime"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// Thresholds at which a sensation begins and at which it is overwhelming. Each
// feeling ramps between the two, so it appears gradually rather than snapping on.
const (
	energyComfortable = 1600.0 // kcal in reserve
	energyStarving    = 400.0

	hydrationComfortable = 99.3 // percent of total body water
	hydrationParched     = 96.5

	bladderNoticeable = 45.0 // percent full
	bladderUrgent     = 95.0

	bowelNoticeable = 40.0
	bowelUrgent     = 90.0

	// Air hunger is driven far more by carbon dioxide than by oxygen: a diver
	// holding their breath feels the urge from rising CO₂ long before oxygen runs
	// short, which is why hyperventilating first is dangerous. Both are smoothed
	// across breaths.
	o2Comfortable = 80.0 // mmHg PaO₂
	o2Alarming    = 45.0

	co2Comfortable = 45.0 // mmHg PaCO₂
	co2Alarming    = 60.0

	// bloodGasHalfLifeSec smooths the per-breath swing away before it is judged.
	// Chemoreceptors respond to a sustained level, not to the peak of each
	// breath; without this, every feeling would flicker at the breathing rate.
	bloodGasHalfLifeSec = 5.0

	glycemiaComfortable = 75.0 // mg/dL; below this a headache sets in
	glycemiaAlarming    = 45.0

	// Fever starts above the warmth of ordinary exertion. Hard exercise settles
	// near 38.2 C, and calling that a fever would mean the body reported itself
	// ill every time it ran.
	temperatureComfortable = 38.4 // °C
	temperatureFeverish    = 40.0

	exertionComfortable = 2.0 // multiples of resting metabolism
	exertionPunishing   = 9.0

	// contentCeiling is how strong contentment can get. It is capped below 1 so
	// a body at rest reads as quietly fine rather than euphoric.
	contentCeiling = 0.75
)

// GetAffect returns how the body feels about its own condition.
//
// It reads the physiological state and turns numbers into named sensations. It
// is purely interpretive: nothing here changes the body, it only gives what is
// happening a name a person would recognise.
func GetAffect() (*component.Component, error) {
	c, err := component.New("physiology:affect",
		component.WithDescription("Interprets physiological state as named feelings (hungry, thirsty, exhausted...)"),
		component.WithInputs(
			sim.TimePort,
			"body_state",    // hydration, glycemia, energy, temperature
			"venous_blood",  // O2 and CO2 saturation
			"bladder_fill",  //
			"bowel_fill",    //
			"physical_load", // current exertion
			"mental_load",   // arousal and valence
		),
		component.WithOutputs("feelings"),
		component.WithActivationFunc(deriveFeelings),
		component.WithInitialState(func(state component.State) {
			// The last reading of each input is remembered, because they arrive
			// on different mesh cycles; without that, feelings would flicker as
			// each signal came and went.
			state.Set(body.EnergyKcal, StartingEnergyKcal)
			state.Set(body.HydrationPct, 100.0)
			state.Set(body.Glycemia, NormalGlycemia)
			state.Set(body.CoreTemperature, NormalCoreTemperature)
			state.Set(stateO2, bloodstream.NormalPaO2)
			state.Set(stateCO2, bloodstream.NormalPaCO2)
			state.Set(stateBladder, 0.0)
			state.Set(stateBowel, 0.0)
			state.Set(stateExertion, 1.0)
			state.Set(stateArousal, 0.0)
			state.Set(stateValence, 0.0)
			state.Set(stateDt, 0.01)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("physiology:affect: %w", err)
	}
	return c, nil
}

const (
	stateO2       string = "o2_level"
	stateCO2      string = "co2_level"
	stateBladder  string = "bladder_pct"
	stateBowel    string = "bowel_pct"
	stateExertion string = "exertion"
	stateArousal  string = "arousal"
	stateValence  string = "valence"
	stateDt       string = "dt"
)

func deriveFeelings(_ context.Context, this *component.Component) error {
	// The tick arrives on its own cycle, and smoothing needs to know how much
	// time it represents, so record it before anything is folded in.
	if tick := firstSignal(this, sim.TimePort); tick != nil {
		if dt, err := simtime.TickDurationInSec(tick); err == nil {
			this.State().Set(stateDt, dt)
		}
	}

	rememberInputs(this)

	// Feelings are published once per tick, so the set is always internally
	// consistent rather than a mix of readings from different cycles.
	if !this.InputByName(sim.TimePort).HasSignals() {
		return nil
	}

	get := func(key string) float64 { return this.State().Get(key).(float64) }

	feelings := map[string]float64{
		// Falling reserves read as hunger; falling blood sugar as a headache.
		body.FeelingHungry:   ramp(get(body.EnergyKcal), energyComfortable, energyStarving),
		body.FeelingHeadache: ramp(get(body.Glycemia), glycemiaComfortable, glycemiaAlarming),
		body.FeelingThirsty:  ramp(get(body.HydrationPct), hydrationComfortable, hydrationParched),

		body.FeelingNeedToUrinate:  ramp(get(stateBladder), bladderNoticeable, bladderUrgent),
		body.FeelingNeedToDefecate: ramp(get(stateBowel), bowelNoticeable, bowelUrgent),

		// Air hunger comes from either too little oxygen or too much carbon
		// dioxide; whichever is worse is what the body notices.
		body.FeelingBreathless: max(
			ramp(get(stateO2), o2Comfortable, o2Alarming),
			ramp(get(stateCO2), co2Comfortable, co2Alarming),
		),

		body.FeelingFeverish: ramp(get(body.CoreTemperature), temperatureComfortable, temperatureFeverish),
		body.FeelingAnxious:  mathx.Clamp(get(stateArousal)*negativeOnly(get(stateValence)), 0, 1),
		body.FeelingHappy:    mathx.Clamp(get(stateValence), 0, 1),
	}

	// Exhaustion is exertion on top of an empty tank: hard work while well
	// fuelled is merely tiring.
	exertion := ramp(get(stateExertion), exertionComfortable, exertionPunishing)
	feelings[body.FeelingTired] = exertion
	feelings[body.FeelingExhausted] = exertion * feelings[body.FeelingHungry]

	// Contentment is what is left when nothing else is pressing.
	feelings[body.FeelingContent] = contentCeiling * (1 - strongest(feelings))

	return this.OutputByName("feelings").PutSignals(packFeelings(feelings))
}

// rememberInputs stores the latest value of each input, since they arrive on
// different mesh cycles.
func rememberInputs(this *component.Component) {
	if sig := firstSignal(this, "body_state"); sig != nil {
		s := sig.Scalars()
		this.State().Set(body.EnergyKcal, s.ValueOrDefault(body.EnergyKcal, StartingEnergyKcal))
		this.State().Set(body.HydrationPct, s.ValueOrDefault(body.HydrationPct, 100))
		this.State().Set(body.Glycemia, s.ValueOrDefault(body.Glycemia, NormalGlycemia))
		this.State().Set(body.CoreTemperature, s.ValueOrDefault(body.CoreTemperature, NormalCoreTemperature))
	}
	if sig := firstSignal(this, "venous_blood"); sig != nil {
		// Smoothed rather than stored outright, so a feeling reflects how the
		// blood has been rather than where it happened to be mid-breath.
		dt := this.State().Get(stateDt).(float64)
		smooth := func(key string, scalar string, fallback float64) {
			this.State().Update(key, func(v any) any {
				return mathx.DecayToward(v.(float64),
					sig.Scalars().ValueOrDefault(scalar, fallback), dt, bloodGasHalfLifeSec)
			})
		}
		smooth(stateO2, "PaO2", bloodstream.NormalPaO2)
		smooth(stateCO2, "PaCO2", bloodstream.NormalPaCO2)
	}
	if sig := firstSignal(this, "bladder_fill"); sig != nil {
		this.State().Set(stateBladder, sig.Float64OrDefault(0))
	}
	if sig := firstSignal(this, "bowel_fill"); sig != nil {
		this.State().Set(stateBowel, sig.Float64OrDefault(0))
	}
	if sig := firstSignal(this, "physical_load"); sig != nil {
		this.State().Set(stateExertion, sig.Float64OrDefault(1))
	}
	if sig := firstSignal(this, "mental_load"); sig != nil {
		this.State().Set(stateArousal, sig.Scalars().ValueOrDefault("arousal", 0))
		this.State().Set(stateValence, sig.Scalars().ValueOrDefault("valence", 0))
	}
}

func firstSignal(this *component.Component, portName string) *signal.Signal {
	in := this.InputByName(portName)
	if in == nil || !in.HasSignals() {
		return nil
	}
	return in.Signals().First()
}

// packFeelings puts every feeling on one signal, so the whole emotional picture
// travels together and cannot be read half-updated.
func packFeelings(feelings map[string]float64) *signal.Signal {
	sig := signal.New("feelings").WithLabel("category", "affect")
	for _, name := range body.Feelings {
		sig = sig.WithScalar(name, mathx.Clamp(feelings[name], 0, 1))
	}
	return sig
}

// ramp converts a measurement into a 0..1 intensity, where `comfortable` means
// nothing is felt and `extreme` means it is all the body can think about.
//
// It works in either direction, so a feeling driven by a falling value (energy)
// and one driven by a rising value (carbon dioxide) are expressed the same way.
func ramp(value, comfortable, extreme float64) float64 {
	if comfortable == extreme {
		return 0
	}
	return mathx.Clamp((comfortable-value)/(comfortable-extreme), 0, 1)
}

// negativeOnly returns how unpleasant a mood is, ignoring pleasant ones. Arousal
// alone is not anxiety: excitement and dread differ by which way the mood leans.
func negativeOnly(valence float64) float64 {
	return max(-valence, 0)
}

// strongest returns the most intense feeling, ignoring contentment itself.
func strongest(feelings map[string]float64) float64 {
	var peak float64
	for name, intensity := range feelings {
		if name == body.FeelingContent || name == body.FeelingHappy {
			continue
		}
		peak = max(peak, mathx.Clamp(intensity, 0, 1))
	}
	return peak
}
