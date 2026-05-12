package factor

import (
	"errors"

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

// GetGasComponent returns the gas component of the habitat
func GetGasComponent() *component.Component {
	return component.New("gas").
		WithDescription("Gas factor").
		AddInputs("time", "ctl").
		// For the sake of simplicity, we skip parameters like barometric pressure or wind
		AddOutputs("environmental_gas").
		WithInitialState(func(state component.State) {
			// Average air conditions in Valencia
			state.Set("temperature", +26.0)
			state.Set("humidity", 58.8)
		}).
		WithActivationFunc(
			helper.SequentialActivationFunc(
				handleControlSignals,
				emitEnvironmentalGas,
			),
		)
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
	currentTemperature := this.State().Get("temperature").(float64)
	currentHumidity := this.State().Get("humidity").(float64)

	return this.OutputByName("environmental_gas").PutSignals(
		helper.PackAir(nitrogenFraction, oxygenFraction, argonFraction, pollutionFraction, currentTemperature, currentHumidity),
	).ChainableErr()
}
