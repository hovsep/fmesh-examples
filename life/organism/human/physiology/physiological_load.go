package physiology

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/bloodstream"
	"github.com/hovsep/fmesh-examples/life/body"
	da "github.com/hovsep/fmesh-examples/life/organism/human/distributed_anatomy"
	"github.com/hovsep/fmesh-examples/simulation"
	"github.com/hovsep/fmesh-examples/simulation/mathx"
	"github.com/hovsep/fmesh-examples/simulation/simtime"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// DamageTargets are the organs physiological_load can injure. Each is an output
// port here and wires to that organ's damage input.
var DamageTargets = []string{
	"brain", "heart", "diaphragm", "lung_left", "lung_right", "kidney", "liver", "pancreas",
}

// Thresholds at which a reservoir starts to injure organs, and the level at which
// the injury is at full force. Between the two the severity ramps 0..1. These are
// game-level, not clinical, and match this sim's scales.
const (
	// Dehydration (percent of total body water remaining).
	//
	// Read these against body weight, which is how dehydration is actually
	// graded: total body water is about 60% of a person, so losing a tenth of
	// the water is roughly 6% of body weight -- the point at which it stops
	// being thirst and starts being an illness. Full severity is a quarter of
	// the water gone, near 15% of body weight, which is fatal territory.
	//
	// They used to be 97 and 90, which put the onset of organ injury at 3% of
	// body water -- under 2% of body weight, or a hard session in the gym. A
	// body that exercised for two hours without a drink died of it, with every
	// other reading in this table perfectly normal.
	dehydrationOnset = 90.0
	dehydrationFull  = 75.0

	// Oxygen delivery (mL per minute): the content the blood carries times the
	// flow that carries it. A resting adult delivers about 1000 and consumes
	// about 250, so injury begins where the reserve is gone rather than where
	// any one reading looks alarming.
	//
	// This used to be arterial oxygen tension, and tension is a reading rather
	// than a supply -- which is the same mistake the blood itself was corrected
	// for. A body that has lost half its volume has a textbook-normal PaO2 and
	// is dying; so does one whose haemoglobin is bound up with carbon monoxide.
	// Neither registered here at all, so neither could kill anybody: the model
	// watched the gauge instead of the delivery.
	// Calibrated against the haemorrhage classes, which is what makes the ATLS
	// teaching survive contact with this model: a class III bleed bottoms out
	// near 520 mL/min and must leave no injury behind, while half the blood
	// volume bottoms near 360 and must be lethal. The threshold sits between
	// them rather than at a round number.
	deliveryOnset = 500.0
	deliveryFull  = 220.0

	// normalOxygenDelivery is what a resting adult delivers, in mL per minute:
	// about twenty mL per dL carried at five litres a minute.
	normalOxygenDelivery = bloodstream.NormalHemoglobin * bloodstream.HufnerConstant *
		(bloodstream.NormalSaO2 / 100) * 10 * da.RestingCardiacOutput

	// Hypoperfusion (mean arterial pressure, mmHg). Below about 60 the organs
	// that autoregulate can no longer hold their own blood supply, and below 40
	// nothing is being perfused adequately at all. This is the stressor that
	// makes shock lethal rather than merely alarming.
	hypoperfusionOnset = 60.0
	hypoperfusionFull  = 40.0

	// Hypoglycemia (mg/dL).
	hypoglycemiaOnset = 55.0
	hypoglycemiaFull  = 30.0

	// Hyperthermia and hypothermia (°C).
	hyperthermiaOnset = 39.5
	hyperthermiaFull  = 42.5
	hypothermiaOnset  = 34.0
	hypothermiaFull   = 30.0

	// maxDamageRatePerSec is the damage a fully-severe stressor does per second to
	// an organ at sensitivity 1. At that rate an organ fails in a few minutes of
	// extreme stress; lesser sensitivities and severities take proportionally
	// longer. This is game pace, not clinical -- once a reservoir is truly
	// critical, collapse should be watchable, not take hours.
	maxDamageRatePerSec = 1.0 / 240.0
)

// GetPhysiologicalLoad returns the component that turns a body out of balance
// into organ damage.
//
// It reads the reservoirs and the blood and, when any is dangerously out of
// range, injures the organs that depend on it -- with different sensitivities, so
// dehydration, suffocation, starvation and temperature extremes each collapse the
// body in a different order. It is the engine behind the death cascade.
func GetPhysiologicalLoad() (*component.Component, error) {
	c, err := component.New("physiology:physiological_load",
		component.WithDescription("Turns out-of-range reservoirs into organ damage (the death cascade)"),
		component.WithInputs(simulation.TimePort, "body_state", "venous_blood", "map", "cardiac_output"),
		component.WithOutputs(damageOutputs()...),
		component.WithActivationFunc(component.Sequential(
			latchVitals,
			inflictDamage,
		)),
		component.WithInitialState(func(state component.State) {
			state.Set(body.HydrationPct, 100.0)
			state.Set(body.Glycemia, NormalGlycemia)
			state.Set(loadMAP, da.NormalMAP)
			state.Set(body.CoreTemperature, NormalCoreTemperature)
			state.Set(loadDelivery, normalOxygenDelivery)
			state.Set(loadContent, bloodstream.NormalOxygenContent)
			state.Set(loadOutput, da.RestingCardiacOutput)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("physiology:physiological_load: %w", err)
	}
	return c, nil
}

const (
	loadDelivery string = "load_delivery"
	loadContent  string = "load_content"
	loadOutput   string = "load_cardiac_output"
	loadMAP string = "load_map"
)

func damageOutputs() []string {
	outs := make([]string, len(DamageTargets))
	for i, organ := range DamageTargets {
		outs[i] = organ + "_damage"
	}
	return outs
}

// latchVitals remembers the latest reservoir and blood values, since each arrives
// on its own mesh cycle.
func latchVitals(this *component.Component) error {
	if sig := firstSignal(this, "cardiac_output"); sig != nil {
		this.State().Set(loadOutput, signal.AsFloat64OrDefault(sig, da.RestingCardiacOutput))
	}
	if sig := firstSignal(this, "body_state"); sig != nil {
		s := sig.Scalars()
		this.State().Set(body.HydrationPct, s.ValueOrDefault(body.HydrationPct, 100))
		this.State().Set(body.Glycemia, s.ValueOrDefault(body.Glycemia, NormalGlycemia))
		this.State().Set(body.CoreTemperature, s.ValueOrDefault(body.CoreTemperature, NormalCoreTemperature))
	}
	if sig := firstSignal(this, "venous_blood"); sig != nil {
		// Delivery, not tension: what the blood is carrying times what is
		// carrying it. CaO2 is mL per dL and cardiac output is L per min, so ten
		// decilitres to the litre turns the pair into mL per minute.
		content := sig.Scalars().ValueOrDefault("CaO2", bloodstream.NormalOxygenContent)
		this.State().Set(loadContent, content)
		this.State().Set(loadDelivery, content*10*this.State().Get(loadOutput).(float64))
	}
	if sig := firstSignal(this, "map"); sig != nil {
		this.State().Set(loadMAP, signal.AsFloat64OrDefault(sig, da.NormalMAP))
	}
	return nil
}

func inflictDamage(this *component.Component) error {
	tick := this.InputByName(simulation.TimePort).Signals().First()
	if tick == nil {
		return nil
	}
	dt, err := simtime.TickDurationInSec(tick)
	if err != nil {
		return fmt.Errorf("physiological load tick: %w", err)
	}

	get := func(key string) float64 { return this.State().Get(key).(float64) }

	// Severity of each stressor, 0..1.
	dehydration := rampUpAsFalls(get(body.HydrationPct), dehydrationOnset, dehydrationFull)
	hypoxia := rampUpAsFalls(get(loadDelivery), deliveryOnset, deliveryFull)
	hypoglycemia := rampUpAsFalls(get(body.Glycemia), hypoglycemiaOnset, hypoglycemiaFull)
	hypoperfusion := rampUpAsFalls(get(loadMAP), hypoperfusionOnset, hypoperfusionFull)
	hyperthermia := rampUpAsRises(get(body.CoreTemperature), hyperthermiaOnset, hyperthermiaFull)
	hypothermia := rampUpAsFalls(get(body.CoreTemperature), hypothermiaOnset, hypothermiaFull)
	temperature := max(hyperthermia, hypothermia)

	// Per-organ sensitivity to each stressor. Different profiles give the collapse
	// a visible order: the kidney goes first when dehydrated, the brain first when
	// starved of oxygen or sugar.
	// The kidney suffers a failing pressure first and worst: it is given a fifth
	// of the cardiac output precisely so that it can filter, and it is the first
	// bed sacrificed when there is not enough pressure to go round. Acute kidney
	// injury is the classic survivor's complication of a shock that was itself
	// survived.
	damage := map[string]float64{
		// The kidney is hurt by a failing supply harder than anything else,
		// which is the same reason it is hurt by a failing pressure harder: it
		// is given a fifth of the cardiac output to filter with, and it is the
		// first bed the body gives up. It had no sensitivity to delivery at all
		// until delivery was something this model measured, which quietly made
		// it the organ that survived a haemorrhage best.
		"kidney":     2.0*dehydration + 0.3*temperature + 2.0*hypoperfusion + 1.8*hypoxia,
		"brain":      0.8*dehydration + 1.0*hypoxia + 1.0*hypoglycemia + 0.6*temperature + 1.2*hypoperfusion,
		"heart":      0.2*dehydration + 0.8*hypoxia + 0.6*temperature + 1.0*hypoperfusion,
		"diaphragm":  0.5*hypoxia + 0.4*temperature + 0.4*hypoperfusion,
		"lung_left":  0.4 * temperature,
		"lung_right": 0.4 * temperature,

		// The liver is a shock organ. Ischaemic hepatitis -- "shock liver" -- is
		// what a liver does after an hour of low pressure, and it closes a loop
		// worth watching for: a liver hurt by shock stops defending blood sugar,
		// the hypoglycaemia that follows injures the brain, and the brain was
		// already being injured by the same shock. Nothing here arranges that
		// spiral. It is what these numbers do when put next to each other.
		"liver":    1.5*hypoperfusion + 0.8*hypoxia + 0.3*temperature,
		"pancreas": 1.0*hypoperfusion + 0.4*hypoxia,
	}

	for organ, sensitivity := range damage {
		amount := sensitivity * maxDamageRatePerSec * dt
		if amount <= 0 {
			continue
		}
		if err := this.OutputByName(organ + "_damage").PutSignals(
			signal.New(amount).WithLabel("category", "damage"),
		); err != nil {
			return err
		}
	}
	return nil
}

// rampUpAsFalls returns 0 at or above onset and 1 at or below full, ramping up as
// the value falls (for reservoirs that hurt when they run low).
func rampUpAsFalls(value, onset, full float64) float64 {
	if onset == full {
		return 0
	}
	return mathx.Clamp((onset-value)/(onset-full), 0, 1)
}

// rampUpAsRises returns 0 at or below onset and 1 at or above full, ramping up as
// the value rises (for quantities that hurt when they run high).
func rampUpAsRises(value, onset, full float64) float64 {
	if onset == full {
		return 0
	}
	return mathx.Clamp((value-onset)/(full-onset), 0, 1)
}
