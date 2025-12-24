package factor

import (
	"strconv"
	"strings"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// GetTempComponent returns the temperature component of the environment
func GetTempComponent() *component.Component {
	return component.New("temperature").
		WithDescription("Ambient temperature in Celsius degrees").
		WithInitialState(func(state component.State) {
			state.Set("current_temperature", 200)
		}).
		AddInputs("ctl", "time").
		AddOutputs("current_temperature").
		WithActivationFunc(func(this *component.Component) error {

			this.InputByName("ctl").Signals().ForEach(func(sig *signal.Signal) error {
				cmd := sig.PayloadOrNil().(string)
				if strings.HasPrefix(cmd, "set:") {
					newTemp, err := strconv.ParseInt(strings.TrimPrefix(cmd, "set:"), 10, 64)
					if err != nil {
						return err
					}
					this.State().Set("current_temperature", int(newTemp))
				}

				currentTemp := this.State().Get("current_temperature")

				this.OutputByName("current_temperature").PutSignals(signal.New(currentTemp))
				this.Logger().Println("Temperature changed:", currentTemp)
				return nil
			})

			return nil
		})
}
