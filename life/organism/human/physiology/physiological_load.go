package physiology

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// DamageTargets are the organs physiological_load can injure. Each is an output
// port here and wires to that organ's damage input.
var DamageTargets = []string{"brain", "heart", "diaphragm", "lung_left", "lung_right", "kidney"}

// Thresholds at which a reservoir starts to injure organs, and the level at which
// the injury is at full force. Between the two the severity ramps 0..1. These are
// game-level, not clinical, and match this sim's scales.
const (
	// Dehydration (percent of total body water).
	dehydrationOnset = 97.0
	dehydrationFull  = 90.0

	// Hypoxia (blood O2, game scale).
	hypoxiaOnset = 60.0
	hypoxiaFull  = 30.0

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
		component.WithInputs(common.TimePort, "body_state", "venous_blood"),
		component.WithOutputs(damageOutputs()...),
		component.WithActivationFunc(helper.SequentialActivationFunc(
			latchVitals,
			inflictDamage,
		)),
		component.WithInitialState(func(state component.State) {
			state.Set(common.HydrationPct, 100.0)
			state.Set(common.Glycemia, NormalGlycemia)
			state.Set(common.CoreTemperature, NormalCoreTemperature)
			state.Set(loadO2, 100.0)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("physiology:physiological_load: %w", err)
	}
	return c, nil
}

const loadO2 common.State = "load_o2"

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
	if sig := firstSignal(this, "body_state"); sig != nil {
		s := sig.Scalars()
		this.State().Set(common.HydrationPct, s.ValueOrDefault(common.HydrationPct, 100))
		this.State().Set(common.Glycemia, s.ValueOrDefault(common.Glycemia, NormalGlycemia))
		this.State().Set(common.CoreTemperature, s.ValueOrDefault(common.CoreTemperature, NormalCoreTemperature))
	}
	if sig := firstSignal(this, "venous_blood"); sig != nil {
		this.State().Set(loadO2, sig.Scalars().ValueOrDefault("O2_level", 100))
	}
	return nil
}

func inflictDamage(this *component.Component) error {
	tick := this.InputByName(common.TimePort).Signals().First()
	if tick == nil {
		return nil
	}
	dt, err := helper.TickDurationInSec(tick)
	if err != nil {
		return fmt.Errorf("physiological load tick: %w", err)
	}

	get := func(key common.State) float64 { return this.State().Get(key).(float64) }

	// Severity of each stressor, 0..1.
	dehydration := rampUpAsFalls(get(common.HydrationPct), dehydrationOnset, dehydrationFull)
	hypoxia := rampUpAsFalls(get(loadO2), hypoxiaOnset, hypoxiaFull)
	hypoglycemia := rampUpAsFalls(get(common.Glycemia), hypoglycemiaOnset, hypoglycemiaFull)
	hyperthermia := rampUpAsRises(get(common.CoreTemperature), hyperthermiaOnset, hyperthermiaFull)
	hypothermia := rampUpAsFalls(get(common.CoreTemperature), hypothermiaOnset, hypothermiaFull)
	temperature := max(hyperthermia, hypothermia)

	// Per-organ sensitivity to each stressor. Different profiles give the collapse
	// a visible order: the kidney goes first when dehydrated, the brain first when
	// starved of oxygen or sugar.
	damage := map[string]float64{
		"kidney":     2.0*dehydration + 0.3*temperature,
		"brain":      0.8*dehydration + 1.0*hypoxia + 1.0*hypoglycemia + 0.6*temperature,
		"heart":      0.2*dehydration + 0.8*hypoxia + 0.6*temperature,
		"diaphragm":  0.5*hypoxia + 0.4*temperature,
		"lung_left":  0.4 * temperature,
		"lung_right": 0.4 * temperature,
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
	return helper.Clamp((onset-value)/(onset-full), 0, 1)
}

// rampUpAsRises returns 0 at or below onset and 1 at or above full, ramping up as
// the value rises (for quantities that hurt when they run high).
func rampUpAsRises(value, onset, full float64) float64 {
	if onset == full {
		return 0
	}
	return helper.Clamp((value-onset)/(full-onset), 0, 1)
}
