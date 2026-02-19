package factor

import (
	"errors"
	"fmt"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// GetAirComponent returns the air component of the habitat
func GetAirComponent() *component.Component {
	return component.New("air").
		WithDescription("Air factor").
		AddInputs("time", "ctl").
		// For the sake of simplicity, we skip parameters like barometric pressure or wind
		AddOutputs("temperature", "composition", "humidity").
		WithInitialState(func(state component.State) {
			// Average air conditions in Valencia
			state.Set("temperature", +26.0)
			state.Set("humidity", 58.8)
			state.Set("composition", getDefaultAirComposition())
		}).
		WithActivationFunc(func(this *component.Component) error {
			err := handleControlSignals(this)
			if err != nil {
				return fmt.Errorf("failed to handle control signals: %v", err)
			}

			return emitAir(this)
		})
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
				return currentTemp.(float64) + ctlSig.PayloadOrDefault(0.0).(float64)
			})
			return nil
		case "set_temperature":
			this.Logger().Println("Setting temperature to ", ctlSig.PayloadOrDefault(0.0).(float64))
			this.State().Update("temperature", func(currentTemp any) any {
				return ctlSig.PayloadOrDefault(0.0).(float64)
			})
			return nil
		default:
			return errors.New("unknown command")
		}

	})

	return nil
}

func emitAir(this *component.Component) error {
	currentTemperature := this.State().Get("temperature").(float64)
	currentHumidity := this.State().Get("humidity").(float64)
	currentComposition := this.State().Get("composition").(*signal.Group)

	this.OutputByName("temperature").PutSignals(signal.New(currentTemperature))
	this.OutputByName("humidity").PutSignals(signal.New(currentHumidity))
	this.OutputByName("composition").PutSignalGroups(currentComposition)

	return nil
}

// getDefaultAirComposition returns the default air composition
// represented as a group of signals.
// Each signal is a float64 value representing the relative amount of a compound.
// The sum of all components must add up to 100%
func getDefaultAirComposition() *signal.Group {
	return signal.NewGroup().Add(
		// Major Atmospheric Components
		signal.New(78.084).AddLabel("name", "nitrogen").AddLabel("alias", "N2"),
		signal.New(20.946).AddLabel("name", "oxygen").AddLabel("alias", "O2"),
		signal.New(0.934).AddLabel("name", "argon").AddLabel("alias", "Ar"),

		// Trace Greenhouse Gases & Compounds
		signal.New(0.0421).AddLabel("name", "carbon_dioxide").AddLabel("alias", "CO2"), // Global 2026 avg approx
		signal.New(0.0018).AddLabel("name", "neon").AddLabel("alias", "Ne"),

		// Pollutants
		signal.New(0.000035).AddLabel("name", "carbon_monoxide").AddLabel("alias", "CO"),
		signal.New(0.000004).AddLabel("name", "ozone").AddLabel("alias", "O3"), // Ground level ozone
		signal.New(0.0000006).AddLabel("name", "nitrogen_dioxide").AddLabel("alias", "NO2"),
		signal.New(0.0000001).AddLabel("name", "particulate_matter").AddLabel("alias", "PM2.5"),
		signal.New(0.00000001).AddLabel("name", "toxics").AddLabel("alias", "HAP"), // Hazardous air pollutants
	)
}
