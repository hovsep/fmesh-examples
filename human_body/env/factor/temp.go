package factor

import (
	"errors"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// GetTemperatureComponent returns the temperature component of the environment
func GetTemperatureComponent() *component.Component {
	return component.New("temperature").
		WithDescription("Ambient temperature in Celsius degrees").
		WithInitialState(func(state component.State) {
			state.Set("current_temperature", +05.0)
		}).
		AddInputs("time", "ctl").
		AddOutputs("current_temperature").
		WithActivationFunc(func(this *component.Component) error {
			if this.InputByName("ctl").HasSignals() {
				this.InputByName("ctl").Signals().ForEach(func(ctlSig *signal.Signal) error {
					if !ctlSig.Labels().Has("cmd") {
						return nil
					}

					switch ctlSig.Labels().ValueOrDefault("cmd", "") {
					case "change_temperature":
						this.State().Update("current_temperature", func(currentTemp any) any {
							return currentTemp.(float64) + ctlSig.PayloadOrDefault(0.0).(float64)
						})
						return nil
					case "set_temperature":
						this.State().Update("current_temperature", func(currentTemp any) any {
							return ctlSig.PayloadOrDefault(0.0).(float64)
						})
						return nil
					default:
						return errors.New("unknown command")
					}
				})
			}

			currentTemperature := this.State().Get("current_temperature").(float64)
			this.OutputByName("current_temperature").PutSignals(
				signal.New(currentTemperature).
					AddLabel("env-factor", "temperature").
					AddLabel("unit", "Celsius degrees"),
			)
			return nil
		})
}
