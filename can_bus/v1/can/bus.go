package can

import (
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
)

// NewBus creates a new CAN bus
func NewBus(name string) *component.Component {
	return component.New("can_bus-"+name).
		WithInputs(PortCANL, PortCANH).
		WithOutputs(PortCANL, PortCANH).
		WithActivationFunc(func(this *component.Component) error {
			errL := port.ForwardSignals(this.InputByName(PortCANL), this.OutputByName(PortCANL))
			errH := port.ForwardSignals(this.InputByName(PortCANH), this.OutputByName(PortCANH))

			if errL != nil {
				return errL
			}

			if errH != nil {
				return errH
			}

			return nil
		})
}
