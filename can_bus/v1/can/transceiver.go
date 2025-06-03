package can

import "github.com/hovsep/fmesh/component"

// NewTransceiver creates a CAN transceiver component
// which converts bits to voltage and vice versa
func NewTransceiver(unitName string) *component.Component {
	return component.New("can_transceiver-"+unitName).
		WithActivationFunc(func(this *component.Component) error {
			return nil
		}).
		WithInputs(PortCANTx, PortCANH, PortCANL). // Bits in (write to bus), voltage in (read from bus)
		WithOutputs(PortCANRx, PortCANH, PortCANL) // Bits out (read from bus), voltage out (write to bus)
}
