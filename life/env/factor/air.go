package factor

import (
	"errors"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// GetAirComponent returns the air component of the habitat
func GetAirComponent() *component.Component {
	return component.New("air").
		WithDescription("Air factor").
		AddInputs("time", "ctl").
		AddOutputs("temperature", "composition", "humidity").
		WithInitialState(func(state component.State) {
			state.Set("temperature", +26.0)
		}).
		WithActivationFunc(func(this *component.Component) error {
			return handleControlSignals(this)
		})
}

func handleControlSignals(this *component.Component) error {
	// Handle commands
	this.InputByName("ctl").
		Signals().
		Filter(func(s *signal.Signal) bool {
			return s.Labels().Has("cmd")
		}).ForEach(func(ctlSig *signal.Signal) error {
		switch ctlSig.Labels().ValueOrDefault("cmd", "") {
		case "change_temperature":
			this.State().Update("temperature", func(temp any) any {
				return temp.(float64) + ctlSig.PayloadOrDefault(0.0).(float64)
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
