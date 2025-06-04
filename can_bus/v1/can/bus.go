package can

import (
	"github.com/hovsep/fmesh/component"
)

// NewBus creates a new CAN bus
func NewBus(name string) *component.Component {
	return component.New("can_bus-"+name).
		WithInputs(PortCANL, PortCANH).
		WithOutputs(PortCANL, PortCANH).
		WithActivationFunc(func(this *component.Component) error {

			// TODO: add in-place noise generator

			// TODO resolve resulting voltages:
			// add validation: drop any pair of voltages if H < L, and set MIN_VALID and MAX_VALID voltages

			// use min(CAN_L)\max(CAN_H) approximation

			return nil
		})
}
