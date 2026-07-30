package factor

//@TODO: shall we rename gas to atmosphere or air ?
//First I called it gas with idea to experiment with different surrounding gases, but let's stick with normal atmosphere or air for simplicity
// If we call it atmosphere we can also simulate rain\snow\clouds

import (
	"errors"
	"fmt"

	"github.com/hovsep/fmesh-examples/life/atmosphere"
	"github.com/hovsep/fmesh-examples/life/helper"
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

	// Command verbs.
	cmdSetAltitude = "set_altitude"
	cmdSetCO       = "set_co"
)

//@TODO: let's add a feature called "modes or profiles or presets":
// - the idea: the same factor (gas or sun or other env factors in future like noise) can operate in different modes, example: for gas: sea level atmosphere\ everest peak atmosphere, for sun: mode:Valencia and mode:Oslo will have different UV and other params
// - let's implement it as a plugin which just stores presets in state instead of constants and allows switching them via command sent to ctl port
// - let's add this capability (via plugin) to gas and sun, for time it is questionable

// GetGasComponent returns the gas component of the habitat
func GetGasComponent() (*component.Component, error) {
	c, err := component.New("gas",
		component.WithDescription("Gas factor"),
		component.WithInputs("time", "ctl"),
		// For the sake of simplicity, we skip parameters like barometric pressure or wind
		component.WithOutputs("environmental_gas"),
		component.WithActivationFunc(
			helper.SequentialActivationFunc(
				handleControlSignals,
				emitEnvironmentalGas,
			),
		),
		component.WithInitialState(func(state component.State) {
			// Average air conditions in Valencia, which is at sea level.
			state.Set("temperature", +26.0)
			state.Set("humidity", 58.8)
			state.Set(StateAltitude, 0.0)
			state.Set(StateCOppm, 0.0)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("gas component: %w", err)
	}
	return c, nil
}

// The component can receive control signals and change internal state
func handleControlSignals(this *component.Component) error {
	// Handle commands
	this.InputByName("ctl").
		Signals().
		Filter(func(s *signal.Signal) bool {
			return s.Labels().Has("cmd")
		}).ForEach(func(ctlSig *signal.Signal) error {
		switch ctlSig.Labels().ValueOrDefault("cmd", "") {
		case "change_temperature":
			this.State().Update("temperature", func(currentTemp any) any {
				return currentTemp.(float64) + helper.AsF64OrDefault(ctlSig, 0.0)
			})
			return nil
		case cmdSetAltitude:
			metres := helper.AsF64OrDefault(ctlSig, 0.0)
			this.State().Set(StateAltitude, metres)
			this.Logger().Printf("moved to %.0f m: barometric pressure %.0f mmHg",
				metres, atmosphere.PressureAtAltitude(metres))
			return nil
		case cmdSetCO:
			ppm := max(helper.AsF64OrDefault(ctlSig, 0.0), 0)
			this.State().Set(StateCOppm, ppm)
			this.Logger().Printf("carbon monoxide in the air: %.0f ppm", ppm)
			return nil
		case "set_temperature":
			this.Logger().Println("Setting temperature to ", helper.AsF64OrDefault(ctlSig, 0.0))
			this.State().Update("temperature", func(currentTemp any) any {
				return helper.AsF64OrDefault(ctlSig, 0.0)
			})
			return nil
		default:
			return errors.New("unknown command")
		}

	})

	return nil
}

func emitEnvironmentalGas(this *component.Component) error {
	// Only emit on a time tick. A control signal (e.g. a temperature change) can
	// activate this component out of band; without this guard it would emit an
	// extra, unpaired environmental_gas signal that the human component holds
	// forever (waiting for a matching time tick), preventing the mesh from ever
	// converging.
	if !this.InputByName("time").HasSignals() {
		return nil
	}

	currentTemperature := this.State().Get("temperature").(float64)
	currentHumidity := this.State().Get("humidity").(float64)

	air, err := atmosphere.Pack(nitrogenFraction, oxygenFraction, argonFraction, pollutionFraction, currentTemperature, currentHumidity)
	if err != nil {
		return fmt.Errorf("emit environmental gas: %w", err)
	}

	// Altitude changes the pressure and nothing else. The air on a mountain is
	// still 21% oxygen; there is simply less of it, and every consequence of
	// being up there follows from that one number falling.
	altitude := this.State().Get(StateAltitude).(float64)

	return this.OutputByName("environmental_gas").PutSignals(
		atmosphere.WithCarbonMonoxide(
			atmosphere.WithPressure(air, atmosphere.PressureAtAltitude(altitude)),
			this.State().Get(StateCOppm).(float64)))
}
