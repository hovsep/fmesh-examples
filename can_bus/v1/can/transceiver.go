package can

import (
	"errors"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	DominantBit  = Bit(false)
	RecessiveBit = Bit(true)
)

// NewTransceiver creates a CAN transceiver component
// which converts bits to voltage and vice versa
func NewTransceiver(unitName string) *component.Component {
	return component.New("can_transceiver-"+unitName).
		WithActivationFunc(func(this *component.Component) error {
			// Write path: transceiver -> bus
			for _, sig := range this.InputByName(PortCANTx).AllSignalsOrNil() {
				bit, ok := sig.PayloadOrNil().(Bit)
				if !ok {
					this.Logger().Println("received corrupted bit")
				}

				this.Logger().Println("writing bit", bit.String())

				if bit == DominantBit {
					// Drive dominant
					this.OutputByName(PortCANL).PutSignals(signal.New(DominantLowVoltage))
					this.OutputByName(PortCANH).PutSignals(signal.New(DominantHighVoltage))
				} else {
					// High impedance (recessive bit)
					this.OutputByName(PortCANL).PutSignals(signal.New(RecessiveVoltage))
					this.OutputByName(PortCANH).PutSignals(signal.New(RecessiveVoltage))
				}
			}

			// Read path: transceiver <- bus (exactly one bit)
			if this.InputByName(PortCANL).HasSignals() && this.InputByName(PortCANH).HasSignals() {
				vLow, err := this.InputByName(PortCANL).FirstSignalPayload()
				if err != nil {
					return errors.New("failed to read voltage from L")
				}

				vHigh, err := this.InputByName(PortCANH).FirstSignalPayload()
				if err != nil {
					return errors.New("failed to read voltage from H")
				}

				if vLow == nil || vHigh == nil {
					return errors.New("received invalid voltage")
				}

				bitRead := voltageToBit(vLow.(Voltage), vHigh.(Voltage))
				this.Logger().Println("read bit from bus: ", bitRead.String())
				this.OutputByName(PortCANRx).PutSignals(signal.New(bitRead))
			}

			return nil
		}).
		WithInputs(PortCANTx, PortCANH, PortCANL). // Bits in (write to bus), voltage in (read from bus)
		WithOutputs(PortCANRx, PortCANH, PortCANL) // Bits out (read from bus), voltage out (write to bus)
}
