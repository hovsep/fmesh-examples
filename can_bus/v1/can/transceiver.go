package can

import (
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

type Voltage float64

const (
	DominantBit  = Bit(false)
	RecessiveBit = Bit(true)

	DominantHighVoltage = Voltage(3.7)
	DominantLowVoltage  = Voltage(1.5)

	RecessiveVoltage = Voltage(2.5)
)

// NewTransceiver creates a CAN transceiver component
// which converts bits to voltage and vice versa
func NewTransceiver(unitName string) *component.Component {
	return component.New("can_transceiver-"+unitName).
		WithActivationFunc(func(this *component.Component) error {
			for _, sig := range this.InputByName(PortCANTx).AllSignalsOrNil() {
				bit, ok := sig.PayloadOrNil().(Bit)
				if !ok {
					this.Logger().Println("transceiver received corrupted bit")
				}

				this.Logger().Println("transceiver received bit", bit.String())

				if bit == DominantBit {
					// Drive dominant
					this.OutputByName(PortCANL).PutSignals(signal.New(DominantLowVoltage))
					this.OutputByName(PortCANH).PutSignals(signal.New(DominantHighVoltage))
				} else {
					// High impedance
					this.OutputByName(PortCANL).PutSignals(signal.New(RecessiveVoltage))
					this.OutputByName(PortCANH).PutSignals(signal.New(RecessiveVoltage))
				}
			}
			return nil
		}).
		WithInputs(PortCANTx, PortCANH, PortCANL). // Bits in (write to bus), voltage in (read from bus)
		WithOutputs(PortCANRx, PortCANH, PortCANL) // Bits out (read from bus), voltage out (write to bus)
}
