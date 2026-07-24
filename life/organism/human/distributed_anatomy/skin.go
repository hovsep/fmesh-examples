package da

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	// NormalSkinCoreTemperature is what the skin assumes before it has been told
	// otherwise.
	NormalSkinCoreTemperature = 37.0 * Celsius

	// InsensibleLossMlPerSec is the water a resting body loses through skin and
	// breath without noticing: roughly 700 mL a day.
	InsensibleLossMlPerSec = 700.0 / 86400.0 * Milliliter

	// sweatOnsetTemperature is the core temperature at which sweating begins.
	sweatOnsetTemperature = 37.2 * Celsius

	// sweatMlPerDegreePerSec is how hard the body sweats per degree above onset.
	// Two degrees over comes out near 1.5 L an hour, which is about the most a
	// person can actually sustain.
	sweatMlPerDegreePerSec = 750.0 / 3600.0 * Milliliter
)

// GetSkin returns the skin.
//
// Its job here is water: the steady insensible loss every body has, plus sweat
// once the core runs hot. That makes exertion cost hydration, which is what
// connects running to thirst. Pain and mechanical load are declared but not yet
// modelled.
func GetSkin() (*component.Component, error) {
	c, err := component.New("da:skin",
		component.WithDescription("Skin: loses water steadily, and sweats when the body runs hot"),
		component.WithInputs(
			common.TimePort,
			"body_state", // from physiology:physiological_state
			"thermal_load",
			"radiation",
			"mechanical_load",
		),
		component.WithOutputs(
			"losses", // water leaving the body, to physiology:physiological_state
			"temperature_change",
			"pain_signal",
			"sweat_rate",
		),
		component.WithActivationFunc(loseWater),
		component.WithInitialState(func(state component.State) {
			state.Set(common.CoreTemperature, NormalSkinCoreTemperature)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("da:skin: %w", err)
	}
	return c, nil
}

func loseWater(this *component.Component) error {
	// Remember the core temperature whenever it arrives. It reaches this
	// component on its own cycle, so holding the last value keeps sweating
	// steady rather than switching off on ticks where the signal has not landed.
	if in := this.InputByName("body_state"); in.HasSignals() {
		if sig := in.Signals().First(); sig != nil {
			this.State().Set(common.CoreTemperature,
				sig.Scalars().ValueOrDefault(common.CoreTemperature, NormalSkinCoreTemperature))
		}
	}

	tick := this.InputByName(common.TimePort).Signals().First()
	if tick == nil {
		return nil
	}

	dt, err := helper.TickDurationInSec(tick)
	if err != nil {
		return fmt.Errorf("skin tick: %w", err)
	}

	coreTemperature := this.State().Get(common.CoreTemperature).(float64)
	sweatRate := max(coreTemperature-sweatOnsetTemperature, 0) * sweatMlPerDegreePerSec
	lostMl := (InsensibleLossMlPerSec + sweatRate) * dt

	if err := this.OutputByName("sweat_rate").PutPayloads(sweatRate); err != nil {
		return err
	}
	return this.OutputByName("losses").PutSignals(
		signal.New("losses").
			WithLabel("category", "skin").
			WithScalar(common.WaterMl, lostMl),
	)
}
