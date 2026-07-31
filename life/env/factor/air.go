package factor

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/hovsep/fmesh-examples/life/atmosphere"
	"github.com/hovsep/fmesh-examples/simulation/mathx"
	"github.com/hovsep/fmesh-examples/simulation/simtime"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	// We use constants for simplicity, but in future those params can be moved to reusable gas-profiles (per city\country\planet)
	nitrogenFraction  = 77.6
	oxygenFraction    = 21
	argonFraction     = 1
	pollutionFraction = 0.4
)

// Atmospheric state.
const (
	// StateAltitude is how high the habitat is, in metres above sea level. The
	// atmosphere reports a pressure derived from it rather than a pressure set
	// directly, because that is the thing an atmosphere actually has: you can
	// stand somewhere, and the pressure follows.
	//
	// A barochamber is the other way round -- it has a pressure and no altitude
	// at all -- which is most of what makes the two different components rather
	// than one component with a flag.
	StateAltitude = "altitude_m"

	// StateCOppm is carbon monoxide in the air, parts per million. It is a
	// property of a place -- a fire, a faulty boiler, a running engine in a closed
	// garage -- rather than of a body, which is why it lives out here.
	StateCOppm = "co_ppm"

	// StatePressure is the absolute pressure of the air, in mmHg.
	//
	// Pressure is the thing the body actually feels, and altitude is only one way
	// of setting it. A sealed room has a pressure and no altitude at all, which
	// is why the pressure is stored and the altitude is not derived from it.
	StatePressure = "pressure_mmhg"

	// StateOxygenPct is the oxygen fraction of the mixture, as a percentage.
	// Ordinary air is 21 everywhere; anything else is a room somebody pumped.
	StateOxygenPct = "oxygen_pct"

	// Command verbs.
	cmdSetAltitude = "set_altitude"
	cmdSetCO       = "set_co"
	cmdSetPressure = "set_pressure"
	cmdSetOxygen   = "set_oxygen"
	cmdSetPreset   = "set_preset"
	cmdAddMixin    = "add_mixin"
	cmdClearMixins = "clear_mixins"

	// StateMixins maps each active mixin to the seconds it has left.
	StateMixins = "mixins"
)

// A mixin is something in the air that is not the air: smoke, exhaust, a fire
// two streets away. It is separate from a preset because the two compose -- a
// cigarette lit at altitude is both -- and because a preset says where the body
// is while a mixin says what has got in.
//
// Each carries a duration, since none of them are permanent. That is most of
// what distinguishes smoke from weather.
type mixin struct {
	coPpm     float64 // carbon monoxide added, parts per million
	pollution float64 // percentage points of the mixture that become soot
}

var mixins = map[string]mixin{
	// A lit cigarette puts several hundred ppm into the smoke drawn through it,
	// which is why the standing carboxyhemoglobin of a smoker is measurable.
	"cigarette_smoke": {coPpm: 450, pollution: 2.0},

	// An unventilated fire is the classic domestic poisoning, and it is worse
	// than a cigarette for exactly the reason it kills: nobody is puffing on it,
	// so nobody notices.
	"wood_fire": {coPpm: 800, pollution: 5.0},

	// An engine running in a closed garage. The number is deliberately lethal.
	"car_exhaust": {coPpm: 1500, pollution: 3.0},
}

// MixinNames lists the mixins, sorted, for the command that offers them.
func MixinNames() []string {
	return slices.Sorted(maps.Keys(mixins))
}

// A preset is a place, named. The alternative is asking someone to remember that
// three atmospheres of pure oxygen is 2280 and 100, which is how a demonstration
// becomes a lookup table.
type preset struct {
	pressure    float64
	oxygenPct   float64
	temperature float64
	humidity    float64
}

// Presets worth having in front of an audience.
//
// The last two are the pair to run one after the other. They deliver almost
// exactly the same inspired oxygen tension -- 0.21 x (380 - 47) = 70 mmHg
// against 0.10 x (760 - 47) = 71 -- by opposite means, and the body cannot tell
// them apart, because the alveolar gas equation multiplies pressure by fraction
// and has no way to know which of the two moved. That is the whole principle an
// altitude tent is sold on.
var presets = map[string]preset{
	"sea_level":     {atmosphere.SeaLevelPressure, 21, 26.0, 58.8},
	"everest":       {atmosphere.PressureAtAltitude(8848), 21, -30.0, 20.0},
	"hyperbaric":    {3 * atmosphere.SeaLevelPressure, 100, 21.0, 40.0},
	"hypobaric":     {380, 21, 21.0, 40.0},
	"altitude_tent": {atmosphere.SeaLevelPressure, 10, 21.0, 40.0},
}

// PresetNames lists the presets, sorted, for the command that offers them.
func PresetNames() []string {
	return slices.Sorted(maps.Keys(presets))
}

// GetAirComponent returns the air of the habitat.
func GetAirComponent() (*component.Component, error) {
	c, err := component.New("air",
		component.WithDescription("The air: a pressure, a mixture, a temperature, and whatever has got into it"),
		component.WithInputs(
			"time", "ctl",
			// The sun, by the habitat's naming convention (see env/habitat.go).
			// Nothing wires this by hand; declaring the port is the whole of it.
			"habitat_sun_uvi",
		),
		// For the sake of simplicity, we skip parameters like barometric pressure or wind
		component.WithOutputs("environmental_gas"),
		component.WithActivationFunc(
			component.Sequential(
				handleControlSignals,
				warmInTheSun,
				emitEnvironmentalGas,
			),
		),
		component.WithInitialState(func(state component.State) {
			// Average air conditions in Valencia, which is at sea level.
			state.Set(StateShadeTemperature, +26.0)
			state.Set(StateSunUVI, 0.0)
			state.Set("temperature", +26.0)
			state.Set("humidity", 58.8)
			state.Set(StateAltitude, 0.0)
			state.Set(StatePressure, atmosphere.SeaLevelPressure)
			state.Set(StateOxygenPct, 21.0)
			state.Set(StateCOppm, 0.0)
			state.Set(StateMixins, map[string]float64{})
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("air component: %w", err)
	}
	return c, nil
}

// The component can receive control signals and change internal state
func handleControlSignals(_ context.Context, this *component.Component) error {
	// Handle commands
	this.InputByName("ctl").
		Signals().
		Filter(func(s *signal.Signal) bool {
			return s.Labels().Has("cmd")
		}).ForEach(func(ctlSig *signal.Signal) error {
		switch ctlSig.Labels().ValueOrDefault("cmd", "") {
		case "change_temperature":
			setShade(this, this.State().Get(StateShadeTemperature).(float64)+
				signal.AsFloat64OrDefault(ctlSig, 0.0))
			return nil
		case cmdSetAltitude:
			metres := signal.AsFloat64OrDefault(ctlSig, 0.0)
			this.State().Set(StateAltitude, metres)
			this.State().Set(StatePressure, atmosphere.PressureAtAltitude(metres))
			this.Logger().Printf("moved to %.0f m: barometric pressure %.0f mmHg",
				metres, atmosphere.PressureAtAltitude(metres))
			return nil
		case cmdSetPressure:
			// The floor is not arithmetic tidiness but a safety feature: below
			// the vapour pressure of water at body temperature the alveolar gas
			// equation has no positive term left, and a body would not be short
			// of oxygen so much as boiling.
			pressure := max(signal.AsFloat64OrDefault(ctlSig, atmosphere.SeaLevelPressure), 50.0)
			this.State().Set(StatePressure, pressure)
			this.Logger().Printf("air at %.0f mmHg (%.2f atmospheres)",
				pressure, pressure/atmosphere.SeaLevelPressure)
			return nil
		case cmdSetOxygen:
			oxygen := min(max(signal.AsFloat64OrDefault(ctlSig, 21.0), 1.0), 100.0)
			this.State().Set(StateOxygenPct, oxygen)
			this.Logger().Printf("mixture now %.0f%% oxygen", oxygen)
			return nil
		case cmdAddMixin:
			name := ctlSig.Labels().ValueOrDefault("mixin", "")
			if _, ok := mixins[name]; !ok {
				return fmt.Errorf("unknown mixin %q (have %v)", name, MixinNames())
			}
			seconds := max(signal.AsFloat64OrDefault(ctlSig, 0), 0)
			this.State().Update(StateMixins, func(v any) any {
				active := v.(map[string]float64)
				// Lighting a second cigarette while the first is still going
				// adds to the time rather than restarting it.
				active[name] += seconds
				return active
			})
			this.Logger().Printf("%s in the air for %.0f s", name, seconds)
			return nil
		case cmdClearMixins:
			this.State().Set(StateMixins, map[string]float64{})
			this.Logger().Println("the air is clear again")
			return nil
		case cmdSetPreset:
			name := ctlSig.Labels().ValueOrDefault("preset", "")
			p, ok := presets[name]
			if !ok {
				return fmt.Errorf("unknown preset %q (have %v)", name, PresetNames())
			}
			this.State().Set(StatePressure, p.pressure)
			this.State().Set(StateOxygenPct, p.oxygenPct)
			this.State().Set(StateSunUVI, 0.0)
			setShade(this, p.temperature)
			this.State().Set("humidity", p.humidity)
			this.Logger().Printf("air is now %q: %.0f mmHg, %.0f%% oxygen, %.0f C",
				name, p.pressure, p.oxygenPct, p.temperature)
			return nil
		case cmdSetCO:
			ppm := max(signal.AsFloat64OrDefault(ctlSig, 0.0), 0)
			this.State().Set(StateCOppm, ppm)
			this.Logger().Printf("carbon monoxide in the air: %.0f ppm", ppm)
			return nil
		case "set_temperature":
			shade := signal.AsFloat64OrDefault(ctlSig, 0.0)
			this.Logger().Printf("shade temperature now %.1f C; the sun adds to it", shade)
			setShade(this, shade)
			return nil
		default:
			return errors.New("unknown command")
		}

	})

	return nil
}

const (
	// StateSunUVI is the last UV index the sun reported.
	//
	// It is remembered rather than read where it is used, because it does not
	// arrive on the tick. The sun publishes during a cycle, the signal reaches
	// this component's port at the end of it, and the port is drained on the
	// following cycle -- by which time the next tick has not yet come. Read
	// directly, the sun would appear to be shining only on the cycles nobody was
	// looking, which is how the air came to warm toward the shade and no further.
	StateSunUVI = "sun_uvi"

	// StateShadeTemperature is the air's temperature out of the sun -- the
	// weather, as opposed to the day. It is what "temp:hot" and friends set, because
	// that is what a person means when they say how warm it is somewhere.
	StateShadeTemperature = "shade_temperature"

	// solarAirWarmingPerUVI is how much hotter full sun makes the air, per unit
	// of UV index. At the peak index of 8 that is eight degrees over the shade,
	// which is about the difference a clear midday makes.
	solarAirWarmingPerUVI = 1.0

	// airWarmingHalfLifeSec is how sluggishly the air follows the sun. Ten
	// minutes: enough that dawn warms gradually and dusk cools gradually rather
	// than the temperature snapping to wherever the sun currently is.
	//
	// The lag is the whole reason this is a relaxation and not an assignment.
	// Air has heat capacity; a cloud passing over does not instantly cool a
	// street, and the hottest part of the afternoon is not noon.
	airWarmingHalfLifeSec = 600.0
)

// setShade changes the weather, and the air with it.
//
// The lag below is the sun's, not the thermostat's: air takes time to follow the
// sky, but a command that says the place is now freezing means it is freezing
// now. Relaxing into it instead would make "make it cold" mean "make it cold in
// about ten minutes", which is not what anyone typing it wants -- and it is why
// three minutes in a freezer stopped injuring anybody.
func setShade(this *component.Component, shade float64) {
	this.State().Set(StateShadeTemperature, shade)
	this.State().Set("temperature", shade+solarAirWarmingPerUVI*this.State().Get(StateSunUVI).(float64))
}

// warmInTheSun moves the air toward the temperature the current sunlight would
// eventually hold it at.
//
// This is the first link of the chain the whole habitat exists to demonstrate:
// the sun warms the air, the air warms the body through the skin, and a warm
// enough body sweats. Every step after this one was already built; the sun
// simply never reached the air, so the chain began in the middle.
func warmInTheSun(_ context.Context, this *component.Component) error {
	// Latch the sun whenever it speaks, whichever cycle that is.
	if sun := this.InputByName("habitat_sun_uvi"); sun != nil && sun.HasSignals() {
		this.State().Set(StateSunUVI, signal.AsFloat64OrDefault(sun.Signals().First(), 0))
	}

	tick := this.InputByName("time").Signals().First()
	if tick == nil {
		return nil
	}
	dt, err := simtime.TickDurationInSec(tick)
	if err != nil {
		return fmt.Errorf("air tick: %w", err)
	}

	expireMixins(this, dt)

	uvi := this.State().Get(StateSunUVI).(float64)
	shade := this.State().Get(StateShadeTemperature).(float64)
	target := shade + solarAirWarmingPerUVI*uvi

	this.State().Update("temperature", func(current any) any {
		return mathx.DecayToward(current.(float64), target, dt, airWarmingHalfLifeSec)
	})
	return nil
}

// expireMixins counts every active mixin down and forgets the ones that are
// spent. Smoke clears whether or not anybody opens a window.
func expireMixins(this *component.Component, dt float64) {
	this.State().Update(StateMixins, func(v any) any {
		active := v.(map[string]float64)
		for name, left := range active {
			if left-dt <= 0 {
				delete(active, name)
				continue
			}
			active[name] = left - dt
		}
		return active
	})
}

// inTheAir totals what the active mixins are currently contributing.
func inTheAir(this *component.Component) mixin {
	var total mixin
	for name := range this.State().Get(StateMixins).(map[string]float64) {
		m := mixins[name]
		total.coPpm += m.coPpm
		total.pollution += m.pollution
	}
	return total
}

func emitEnvironmentalGas(_ context.Context, this *component.Component) error {
	// Only emit on a time tick. A control signal (e.g. a temperature change) can
	// activate this component out of band; without this guard it would emit an
	// extra, unpaired environmental_gas signal that the human component holds
	// forever (waiting for a matching time tick), preventing the mesh from ever
	// converging.
	if !this.InputByName("time").HasSignals() {
		return nil
	}

	oxygen := this.State().Get(StateOxygenPct).(float64)

	// Whatever is not oxygen is nitrogen, with the traces kept if there is room
	// for them. Real enriched mixtures use helium in the deep ones, for reasons
	// -- narcosis, density, the work of breathing -- this body has no way to
	// feel, so pretending otherwise would be decoration.
	added := inTheAir(this)

	remaining := 100.0 - oxygen
	argon := min(argonFraction, remaining)
	pollution := min(pollutionFraction+added.pollution, remaining-argon)
	nitrogen := remaining - argon - pollution

	air, err := atmosphere.Pack(nitrogen, oxygen, argon, pollution,
		this.State().Get("temperature").(float64), this.State().Get("humidity").(float64))
	if err != nil {
		return fmt.Errorf("emit environmental gas: %w", err)
	}

	// Altitude changes the pressure and nothing else. The air on a mountain is
	// still 21% oxygen; there is simply less of it, and every consequence of
	// being up there follows from that one number falling.
	return this.OutputByName("environmental_gas").PutSignals(
		atmosphere.WithCarbonMonoxide(
			atmosphere.WithPressure(air, this.State().Get(StatePressure).(float64)),
			this.State().Get(StateCOppm).(float64)+added.coPpm))
}
