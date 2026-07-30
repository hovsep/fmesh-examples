package da

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/atmosphere"
	"github.com/hovsep/fmesh-examples/life/body"
	"github.com/hovsep/fmesh-examples/life/plugin/perfusion"
	"github.com/hovsep/fmesh-examples/simulation"
	"github.com/hovsep/fmesh-examples/simulation/simtime"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	// NormalSkinCoreTemperature is what the skin assumes before it has been told
	// otherwise.
	NormalSkinCoreTemperature = body.NormalCoreTemperature

	// InsensibleLossMlPerSec is the water a resting body loses through skin and
	// breath without noticing: roughly 700 mL a day.
	InsensibleLossMlPerSec = 700.0 / 86400.0

	// sweatOnsetTemperature is the core temperature at which sweating begins.
	sweatOnsetTemperature = 37.2

	// sweatMlPerDegreePerSec is how hard the body sweats per degree above onset.
	// Two degrees over comes out near 1.5 L an hour, which is about the most a
	// person can actually sustain.
	sweatMlPerDegreePerSec = 750.0 / 3600.0

	// The body defends a thermoneutral zone: within roughly thermoneutralTemp ±
	// comfortRange degrees Celsius the ambient temperature is fully compensated
	// and the core does not drift. Only the part of the ambient beyond that band becomes a thermal
	// load, so an ordinary room is harmless while a freezing or blazing one is not.
	thermoneutralTemp = 28.0
	comfortRange      = 12.0

	// ambientCouplingPerSec sets how fast the uncompensated part of the ambient
	// pulls the core. The body's own thermoregulation
	// (physiology:physiological_state) pulls back toward 37, so this only wins in
	// real extremes -- but when it wins it does so at game pace, driving a freezing
	// body to hypothermia in minutes rather than the realistic hour.
	ambientCouplingPerSec = 1.0 / 900 // per degree beyond the comfort band

	// solarHeatingPerUVIPerSec is how much direct sun warms the body per unit of
	// UV index.
	solarHeatingPerUVIPerSec = 0.00002
)

// GetSkin returns the skin.
//
// Two jobs: water and heat. It loses water steadily and sweats when the core
// runs hot, and it is the body's thermal interface with the world -- it turns
// the ambient temperature and sunlight into a heating or cooling rate that the
// core temperature reservoir integrates. That is what makes a cold room or a
// midday sun actually reach the body. Pain and mechanical load are declared but
// not yet modelled.
const SkinO2PerMinute = 12.0

// @TODO: check if skin is connected to all relevant components like thermal boundary and thermo-regulation (if exist)
func GetSkin() (*component.Component, error) {
	c, err := component.New("da:skin",
		component.WithDescription("Skin: loses water, sweats when hot, and couples the body to ambient temperature and sun"),
		component.WithPlugins(
			perfusion.New(perfusion.Config{Organ: "skin", O2PerMinute: SkinO2PerMinute}),
		),
		component.WithInputs(
			simulation.TimePort,
			"body_state",  // from physiology:physiological_state (current core temp)
			"ambient_gas", // habitat air, carrying its temperature
			"radiation",   // sun UV index
			"thermal_load",
			"mechanical_load",
		),
		component.WithOutputs(
			"losses",             // water leaving the body
			"temperature_change", // heating/cooling rate for the core reservoir
			"pain_signal",
			"sweat_rate",
		),
		component.WithActivationFunc(component.Sequential(
			rememberEnvironment,
			regulateSkin,
		)),
		component.WithInitialState(func(state component.State) {
			state.Set(body.CoreTemperature, NormalSkinCoreTemperature)
			state.Set(stateAmbientTemp, NormalSkinCoreTemperature)
			state.Set(stateUVIndex, 0.0)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("da:skin: %w", err)
	}
	return c, nil
}

const (
	stateAmbientTemp string = "ambient_temperature"
	stateUVIndex     string = "uv_index"
)

// rememberEnvironment latches the latest core temperature, ambient temperature
// and sun, since each arrives on its own mesh cycle.
func rememberEnvironment(this *component.Component) error {
	if sig := firstSignal(this, "body_state"); sig != nil {
		this.State().Set(body.CoreTemperature,
			sig.Scalars().ValueOrDefault(body.CoreTemperature, NormalSkinCoreTemperature))
	}
	if sig := firstSignal(this, "ambient_gas"); sig != nil {
		if _, _, _, _, temp, _, err := atmosphere.Unpack(sig); err == nil {
			this.State().Set(stateAmbientTemp, temp)
		}
	}
	if sig := firstSignal(this, "radiation"); sig != nil {
		this.State().Set(stateUVIndex, signal.AsFloat64OrDefault(sig, 0))
	}
	return nil
}

func firstSignal(this *component.Component, portName string) *signal.Signal {
	in := this.InputByName(portName)
	if in == nil || !in.HasSignals() {
		return nil
	}
	return in.Signals().First()
}

func regulateSkin(this *component.Component) error {
	tick := this.InputByName(simulation.TimePort).Signals().First()
	if tick == nil {
		return nil
	}

	dt, err := simtime.TickDurationInSec(tick)
	if err != nil {
		return fmt.Errorf("skin tick: %w", err)
	}

	core := this.State().Get(body.CoreTemperature).(float64)
	ambient := this.State().Get(stateAmbientTemp).(float64)
	uvi := this.State().Get(stateUVIndex).(float64)

	// Water: steady insensible loss plus sweat once hot.
	sweatRate := max(core-sweatOnsetTemperature, 0) * sweatMlPerDegreePerSec
	lostMl := (InsensibleLossMlPerSec + sweatRate) * dt

	// Heat: only the part of the ambient beyond the thermoneutral band is a load;
	// the sun adds warmth; sweat cools. The result is a rate the core reservoir
	// integrates. Sweating hard in the heat is what keeps the core from running away.
	thermalRate := ambientLoad(ambient)*ambientCouplingPerSec +
		uvi*solarHeatingPerUVIPerSec -
		sweatRate*sweatCoolingPerMlPerSec

	if err := this.OutputByName("sweat_rate").PutPayloads(sweatRate); err != nil {
		return err
	}
	if err := this.OutputByName("temperature_change").PutPayloads(thermalRate); err != nil {
		return err
	}
	return this.OutputByName("losses").PutSignals(
		signal.New("losses").
			WithLabel("category", "skin").
			WithScalar(body.WaterMl, lostMl),
	)
}

// sweatCoolingPerMlPerSec is how much each mL/s of sweat cools the core. Chosen
// so that maximal sweating offsets a warm environment rather than freezing the
// body.
const sweatCoolingPerMlPerSec = 0.006

// ambientLoad returns the part of the ambient temperature the body cannot fully
// compensate: zero within the thermoneutral band, and the excess beyond it
// (signed) outside it.
func ambientLoad(ambient float64) float64 {
	switch delta := ambient - thermoneutralTemp; {
	case delta > comfortRange:
		return delta - comfortRange
	case delta < -comfortRange:
		return delta + comfortRange
	default:
		return 0
	}
}
