package da

import (
	"context"
	"fmt"

	"github.com/hovsep/fmesh-examples/simulation/life/atmosphere"
	"github.com/hovsep/fmesh-examples/simulation/life/body"
	"github.com/hovsep/fmesh-examples/simulation/life/plugin/perfusion"
	"github.com/hovsep/fmesh-examples/simulation/sim"
	"github.com/hovsep/fmesh-examples/simulation/sim/simtime"
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

	// MaxSweatMlPerSec is the ceiling on that: about two litres an hour. Sweat
	// glands saturate, and a body that could sweat without limit could survive
	// any oven, because the cooling term would simply grow to match. It is the
	// saturation that makes heatstroke possible.
	MaxSweatMlPerSec = 2000.0 / 3600.0

	// shiverOnsetTemperature is the core temperature at which shivering starts.
	// It is close to normal: shivering is an early defence, well before anything
	// a person would call hypothermia.
	shiverOnsetTemperature = 36.6

	// shiverWarmingPerDegreePerSec is how hard the body shivers per degree below
	// onset, and MaxShiverWarmingPerSec is where it saturates -- roughly the
	// point at which involuntary muscle work has doubled resting heat
	// production, which is about all shivering can do.
	shiverWarmingPerDegreePerSec = 0.004 / 1.5
	MaxShiverWarmingPerSec       = 0.004

	// The body defends a thermoneutral zone: within it the ambient temperature
	// is fully compensated and the core does not drift. Only the part of the
	// ambient beyond the band becomes a thermal load.
	//
	// The band is not symmetric, because the two defences are not. Cold is met
	// with insulation and vasoconstriction, which are free and are not modelled
	// separately, so the band absorbs them and reaches down to about 16 C. Heat
	// has one real answer, and this component already models it explicitly a few
	// lines below: sweating. Letting the band absorb hot ambient counted that
	// answer a second time, and the visible result was a body that ignored heat
	// altogether -- fifty simulated minutes at 38 C left the core at exactly
	// 37.000 and the sweat rate at exactly 0.000, so `temp:hot` was a command
	// that provably moved nothing. The hot edge now stops near skin temperature,
	// where passive loss stops, and everything above it is the sweat's problem.
	thermoneutralTemp = 28.0
	comfortRangeCold  = 12.0 // down to 16 C
	comfortRangeHot   = 4.0  // up to 32 C

	// ambientCouplingPerSec sets how fast the uncompensated part of the ambient
	// pulls the core. The body's own thermoregulation
	// (physiology:physiological_state) pulls back toward 37, so this only wins in
	// real extremes -- and when it wins it does so at game pace, faster than life
	// so that a scenario is watchable rather than something to leave running.
	//
	// Game pace, not no pace. At 1/900 a naked body at -35 lost seven degrees of
	// core in two minutes and was dead in eight, which is not watchable so much
	// as instantaneous -- and about a hundred times life. A quarter of that puts
	// hypothermia at a quarter of an hour: still far quicker than the real
	// couple of hours, but long enough to see coming and do something about.
	ambientCouplingPerSec = 1.0 / 3600 // per degree beyond the comfort band

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

func GetSkin() (*component.Component, error) {
	c, err := component.New("da:skin",
		component.WithDescription("Skin: loses water, sweats when hot, and couples the body to ambient temperature and sun"),
		component.WithPlugins(
			perfusion.New(perfusion.Config{Organ: "skin", O2PerMinute: SkinO2PerMinute}),
		),
		component.WithInputs(
			sim.TimePort,
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
			"shiver_rate",
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
func rememberEnvironment(_ context.Context, this *component.Component) error {
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
		this.State().Set(stateUVIndex, sig.Float64OrDefault(0))
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

func regulateSkin(_ context.Context, this *component.Component) error {
	tick := this.InputByName(sim.TimePort).Signals().First()
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
	sweatRate := SweatRateAt(core)
	lostMl := (InsensibleLossMlPerSec + sweatRate) * dt

	// Heat: only the part of the ambient beyond the thermoneutral band is a load;
	// the sun adds warmth; sweat cools and shivering warms. The result is a rate
	// the core reservoir integrates, and the two effectors are what keep it from
	// running away in either direction.
	shiverRate := ShiverWarmingAt(core)
	thermalRate := ambientLoad(ambient)*ambientCouplingPerSec +
		uvi*solarHeatingPerUVIPerSec -
		sweatRate*sweatCoolingPerMlPerSec +
		shiverRate

	if err := this.OutputByName("sweat_rate").PutPayloads(sweatRate); err != nil {
		return err
	}
	if err := this.OutputByName("shiver_rate").PutPayloads(shiverRate); err != nil {
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
	case delta > comfortRangeHot:
		return delta - comfortRangeHot
	case delta < -comfortRangeCold:
		return delta + comfortRangeCold
	default:
		return 0
	}
}

// SweatRateAt returns how fast a body at this core temperature sweats, in mL/s.
func SweatRateAt(core float64) float64 {
	return min(max(core-sweatOnsetTemperature, 0)*sweatMlPerDegreePerSec, MaxSweatMlPerSec)
}

// ShiverWarmingAt returns the heating rate shivering contributes at this core
// temperature, in degrees per second. It is the cold-side mirror of the sweat
// rate: an effector that answers the disturbance rather than a constant that
// hides it.
func ShiverWarmingAt(core float64) float64 {
	return min(max(shiverOnsetTemperature-core, 0)*shiverWarmingPerDegreePerSec, MaxShiverWarmingPerSec)
}
