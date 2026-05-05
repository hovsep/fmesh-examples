package factor

import (
	"errors"

	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
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
			state.Set("composition", getDefaultComposition())
		}).
		WithActivationFunc(
			helper.Pipeline(
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
	currentComposition := this.State().Get("composition").(*signal.Group)

	this.OutputByName("environmental_gas").PutPayloads(
		signal.NewGroup().Add(
			helper.NewLevel(currentTemperature, "temperature"),
			helper.NewLevel(currentHumidity, "humidity"),
			signal.New(currentComposition).AddLabel("param", "composition"),
		))

	return nil
}

// getDefaultComposition returns the default composition of the gas
func getDefaultComposition() *signal.Group {
	return signal.NewGroup().Add(
		// Major Atmospheric Components
		helper.NewLevel(78.0, "nitrogen").AddLabel("formula", "N2"),
		helper.NewLevel(21.0, "oxygen").AddLabel("formula", "O2"),
		helper.NewLevel(1.0, "argon").AddLabel("formula", "Ar"),
	)
}
