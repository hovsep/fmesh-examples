package can

import (
	"errors"

	"github.com/hovsep/fmesh/component"
)

// NewController creates a CAN controller
// which converts frames to bits and vice versa
func NewController(unitName string) *component.Component {
	return component.New("can_controller-"+unitName).
		WithActivationFunc(func(this *component.Component) error {

			// TODO (write path):
			// put frame in a queue (converted to bits)
			// continue transmitting from the latest bit and compare with latest read bit, check if arbitration is lost and retry

			// Write path (mcu writes to bus)
			for _, sig := range this.InputByName(PortCANTx).AllSignalsOrNil() {
				frame, ok := sig.PayloadOrNil().(*Frame)
				if !ok || !frame.isValid() {
					return errors.New("controller received corrupted frame")
				}

				bits := frame.toBits().String()
				_ = bits
			}
			return nil
		}).
		WithInputs(PortCANTx, PortCANRx). // Frame in, bits in
		WithOutputs(PortCANTx, PortCANRx) // Bits out, frame out
}
