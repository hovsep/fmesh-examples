package can

import (
	"errors"
	"fmt"

	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/codec"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/common"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/physical"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// NewTransceiver creates a stateless CAN transceiver component
// which converts bits to voltage and vice versa
func NewTransceiver(unitName string) *component.Component {
	return component.New("can_transceiver-"+unitName).
		AddInputs(common.PortCANTx, common.PortCANH, common.PortCANL).  // Bits in (write to bus), voltage in (read from bus)
		AddOutputs(common.PortCANRx, common.PortCANH, common.PortCANL). // Bits out (read from bus), voltage out (write to bus)
		WithLogger(common.NewNoopLogger()).
		WithActivationFunc(func(this *component.Component) error {
			err := handleTxPath(this)
			if err != nil {
				return fmt.Errorf("failed to handle tx path: %w", err)
			}

			err = handleRxPath(this)
			if err != nil {
				return fmt.Errorf("failed to handle rx path: %w", err)
			}
			return nil
		})
}

// Write path: transceiver -> bus
func handleTxPath(this *component.Component) error {
	return this.InputByName(common.PortCANTx).Signals().ForEach(func(sig *signal.Signal) error {
		bit, ok := sig.PayloadOrNil().(codec.Bit)
		if !ok {
			this.Logger().Println("received corrupted bit")
		}

		// High impedance by default (recessive bit)
		resultingLVoltage, resultingHVoltage := physical.RecessiveVoltage, physical.RecessiveVoltage

		if bit == codec.ProtocolDominantBit {
			// Drive dominant
			resultingLVoltage, resultingHVoltage = physical.DominantLowVoltage, physical.DominantHighVoltage
		}

		this.OutputByName(common.PortCANL).PutSignals(signal.New(resultingLVoltage))
		this.OutputByName(common.PortCANH).PutSignals(signal.New(resultingHVoltage))

		this.Logger().Printf("convert bit: %s to voltages L:%v / H:%v", bit, resultingLVoltage, resultingHVoltage)
		return nil
	}).ChainableErr()
}

// Read path: transceiver <- bus (exactly one bit)
func handleRxPath(this *component.Component) error {
	if this.InputByName(common.PortCANL).HasSignals() && this.InputByName(common.PortCANH).HasSignals() {
		vLow, err := this.InputByName(common.PortCANL).Signals().FirstPayload()
		if err != nil {
			return errors.New("failed to read voltage from L")
		}

		vHigh, err := this.InputByName(common.PortCANH).Signals().FirstPayload()
		if err != nil {
			return errors.New("failed to read voltage from H")
		}

		if vLow == nil || vHigh == nil {
			return errors.New("received invalid voltage")
		}

		bitRead := physical.VoltageToBit(vLow.(physical.Voltage), vHigh.(physical.Voltage))
		this.Logger().Printf("convert voltages L:%v / H:%v to bit: %s", vLow, vHigh, bitRead)
		this.OutputByName(common.PortCANRx).PutSignals(signal.New(bitRead))
	}
	return nil
}
