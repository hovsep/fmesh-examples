package da

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh-examples/life/plugin/perfusion"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// GI tract state: what is in the stomach, and what has worked its way to the
// far end.
const (
	StateStomachKcal common.State = "stomach_kcal"
	StateStomachMl   common.State = "stomach_ml"
	StateBowelPct    common.State = "bowel_fill_pct"
)

const (
	// StomachCapacityMl is a comfortably full stomach.
	StomachCapacityMl = 1000.0 * Milliliter

	// Half-lives for gastric emptying. Water leaves quickly, a meal takes hours.
	// That delay is the point: eating should not top the body up instantly, it
	// should be something the body feels arriving.
	waterEmptyingHalfLifeSec = 10 * 60.0
	foodEmptyingHalfLifeSec  = 90 * 60.0

	// residuePctPerKcal is how much of a digested meal becomes bowel content.
	residuePctPerKcal = 0.012

	// mealVolumeMlPerKcal converts undigested energy back into the space it takes up.
	mealVolumeMlPerKcal = 1.0
)

// GetGITract returns the gut.
//
// It holds what has been swallowed, releases it into the body over time, and
// accumulates the residue that eventually needs voiding.
// GIO2PerMinute is the gut's own resting oxygen demand, mL/min.
//
// Published figures usually quote the splanchnic bed as a whole -- gut and liver
// together, around 60 mL/min -- and this used to carry all of it, because there
// was no liver to carry the rest. There is now, so the gut keeps only its own
// share.
const GIO2PerMinute = 20.0

func GetGITract() (*component.Component, error) {
	c, err := component.New("da:gi_tract",
		component.WithDescription("GI tract: holds swallowed food and water, absorbing them into the body over time"),
		component.WithPlugins(
			// The splanchnic circulation is generous at rest and is the first
			// thing sympathetic tone shuts down when blood has to be found
			// elsewhere.
			perfusion.New(perfusion.Config{Organ: "gi_tract", O2PerMinute: GIO2PerMinute}),
		),
		component.WithInputs(
			common.TimePort,
			"nutrient_load",  // from boundary:ingestion
			"hydration_load", // from boundary:ingestion
			"void",           // from controller:excretion
		),
		component.WithOutputs(
			"absorption", // per-tick gains handed to physiology:physiological_state
			"stomach_fill",
			"bowel_fill",
		),
		component.WithActivationFunc(helper.SequentialActivationFunc(
			acceptSwallowed,
			voidBowel,
			digest,
		)),
		component.WithInitialState(func(state component.State) {
			state.Set(StateStomachKcal, 0.0)
			state.Set(StateStomachMl, 0.0)
			state.Set(StateBowelPct, 0.0)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("da:gi_tract: %w", err)
	}
	return c, nil
}

// acceptSwallowed adds anything ingested this tick to the stomach.
func acceptSwallowed(this *component.Component) error {
	for _, portName := range []string{"nutrient_load", "hydration_load"} {
		in := this.InputByName(portName)
		if !in.HasSignals() {
			continue
		}

		if err := in.Signals().ForEach(func(sig *signal.Signal) error {
			this.State().Update(StateStomachKcal, func(v any) any {
				return v.(float64) + sig.Scalars().ValueOrDefault(common.GlucoseKcal, 0)
			})
			this.State().Update(StateStomachMl, func(v any) any {
				return v.(float64) + sig.Scalars().ValueOrDefault(common.WaterMl, 0)
			})
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func voidBowel(this *component.Component) error {
	if this.InputByName("void").HasSignals() {
		this.State().Set(StateBowelPct, 0.0)
	}
	return nil
}

// digest moves a share of the stomach's contents into the body each tick, and
// leaves residue behind.
func digest(this *component.Component) error {
	tick := this.InputByName(common.TimePort).Signals().First()
	if tick == nil {
		return nil
	}

	dt, err := helper.TickDurationInSec(tick)
	if err != nil {
		return fmt.Errorf("gi tract tick: %w", err)
	}

	kcal := this.State().Get(StateStomachKcal).(float64)
	water := this.State().Get(StateStomachMl).(float64)

	// What is left after this tick's emptying; the difference crossed into the
	// body. Exponential emptying releases a big meal faster at first and tails
	// off, which is how a stomach actually behaves.
	kcalLeft := helper.DecayToward(kcal, 0, dt, foodEmptyingHalfLifeSec)
	waterLeft := helper.DecayToward(water, 0, dt, waterEmptyingHalfLifeSec)

	absorbedKcal := kcal - kcalLeft
	absorbedWater := water - waterLeft

	this.State().Set(StateStomachKcal, kcalLeft)
	this.State().Set(StateStomachMl, waterLeft)

	if absorbedKcal > 0 {
		this.State().Update(StateBowelPct, func(v any) any {
			return helper.Clamp(v.(float64)+absorbedKcal*residuePctPerKcal, 0, 100)
		})
	}

	if absorbedKcal > 0 || absorbedWater > 0 {
		if err := this.OutputByName("absorption").PutSignals(
			signal.New("absorption").
				WithLabel("category", "digestion").
				WithScalar(common.GlucoseKcal, absorbedKcal).
				WithScalar(common.WaterMl, absorbedWater),
		); err != nil {
			return err
		}
	}

	stomachPct := helper.Clamp((waterLeft+kcalLeft*mealVolumeMlPerKcal)/StomachCapacityMl*100, 0, 100)
	if err := this.OutputByName("stomach_fill").PutPayloads(stomachPct); err != nil {
		return err
	}
	return this.OutputByName("bowel_fill").PutPayloads(this.State().Get(StateBowelPct).(float64))
}
