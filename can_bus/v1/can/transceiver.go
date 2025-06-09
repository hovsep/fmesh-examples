package can

import (
	"errors"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// NewTransceiver creates a CAN transceiver component
// which converts bits to voltage and vice versa
func NewTransceiver(unitName string) *component.Component {
	return component.New("can_transceiver-"+unitName).
		WithInputs(PortCANTx, PortCANH, PortCANL).  // Bits in (write to bus), voltage in (read from bus)
		WithOutputs(PortCANRx, PortCANH, PortCANL). // Bits out (read from bus), voltage out (write to bus)
		WithLogger(NewNoopLogger()).
		WithActivationFunc(func(this *component.Component) error {
			// Write path: transceiver -> bus
			for _, sig := range this.InputByName(PortCANTx).AllSignalsOrNil() {
				bit, ok := sig.PayloadOrNil().(Bit)
				if !ok {
					this.Logger().Println("received corrupted bit")
				}

				// High impedance by default (recessive bit)
				resultingLVoltage, resultingHVoltage := RecessiveVoltage, RecessiveVoltage

				if bit == ProtocolDominantBit {
					// Drive dominant
					resultingLVoltage, resultingHVoltage = DominantLowVoltage, DominantHighVoltage
				}

				this.OutputByName(PortCANL).PutSignals(signal.New(resultingLVoltage))
				this.OutputByName(PortCANH).PutSignals(signal.New(resultingHVoltage))

				this.Logger().Printf("convert bit: %s to voltages L:%v / H:%v", bit, resultingLVoltage, resultingHVoltage)
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
				this.Logger().Printf("convert voltages L:%v / H:%v to bit: %s", vLow, vHigh, bitRead)
				this.OutputByName(PortCANRx).PutSignals(signal.New(bitRead))
			}

			return nil
		})
}
