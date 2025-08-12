package bus

import (
	"errors"
	"fmt"
	"slices"

	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/codec"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/common"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/physical"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	// How many recessive bits to emit on bus startup
	initialRecessiveBitsRequest = codec.ProtocolEOFSize + codec.ProtocolIFSSize + 1
)

func newWires(name string) *component.Component {
	wires := component.New(name).
		WithDescription("Simulates differential low/high pair, performs wire-and logic").
		WithInputs(common.PortCANL, common.PortCANH, portRecessiveBitRequest).
		WithOutputs(common.PortCANL, common.PortCANH, portRecessiveBitRequest).
		WithLogger(common.NewNoopLogger()).
		WithActivationFunc(func(this *component.Component) error {
			allLow, allHigh, err := processRecessiveBitRequest(this)
			if err != nil {
				return fmt.Errorf("failed to process recessive bits request: %w", err)
			}

			allLow, err = collectLow(this, allLow)
			if err != nil {
				return fmt.Errorf("failed to collect voltages on CAN_L: %w", err)
			}

			allHigh, err = collectHigh(this, allHigh)
			if err != nil {
				return fmt.Errorf("failed to collect voltages on CAN_H: %w", err)
			}

			err = validateVoltages(allLow, allHigh)
			if err != nil {
				return err
			}

			return doWiredAND(this, allLow, allHigh)
		})

	// Setup self-activation pipe
	wires.OutputByName(portRecessiveBitRequest).PipeTo(wires.InputByName(portRecessiveBitRequest))

	// Initially drive the bus with 11 recessive bits to simulate passive idle state,
	// ensuring all CAN controllers detect bus idle condition.
	wires.InputByName(portRecessiveBitRequest).PutSignals(signal.New(initialRecessiveBitsRequest))

	return wires
}

// Process recessive bit request (the very first idle state, when bus is just powered or control signal from watchdog)
func processRecessiveBitRequest(this *component.Component) ([]physical.Voltage, []physical.Voltage, error) {
	var allLow, allHigh []physical.Voltage

	recessivesCount := this.InputByName(portRecessiveBitRequest).FirstSignalPayloadOrDefault(0).(int)
	if recessivesCount > 0 {
		allLow = append(allLow, physical.RecessiveVoltage)
		allHigh = append(allHigh, physical.RecessiveVoltage)

		// Self-activate
		if recessivesCount > 1 {
			this.OutputByName(portRecessiveBitRequest).PutSignals(signal.New(recessivesCount - 1))
		}
	}
	return allLow, allHigh, nil
}

// Collect CAN_L voltages
func collectLow(this *component.Component, allLow []physical.Voltage) ([]physical.Voltage, error) {
	for _, sig := range this.InputByName(common.PortCANL).AllSignalsOrNil() {
		v, ok := sig.PayloadOrNil().(physical.Voltage)
		if !ok {
			this.Logger().Println("bus received corrupted voltage on CAN_L wire")
		}

		allLow = append(allLow, v)
	}
	return allLow, nil
}

// Collect CAN_H voltages
func collectHigh(this *component.Component, allHigh []physical.Voltage) ([]physical.Voltage, error) {
	for _, sig := range this.InputByName(common.PortCANH).AllSignalsOrNil() {
		v, ok := sig.PayloadOrNil().(physical.Voltage)
		if !ok {
			this.Logger().Println("bus received corrupted voltage on CAN_H wire")
		}

		allHigh = append(allHigh, v)
	}
	return allHigh, nil
}

func validateVoltages(allLow, allHigh []physical.Voltage) error {
	// Basic validations:
	if len(allLow) == 0 {
		return errors.New("no voltage on L")
	}

	if len(allHigh) == 0 {
		return errors.New("no voltage on H")
	}

	if len(allLow) != len(allHigh) {
		return errors.New("voltages count mismatch")
	}

	// Detect faulty transceivers
	for i := 0; i < len(allLow); i++ {
		if allLow[i] > allHigh[i] {
			return errors.New("voltage on L is higher than on H")
		}

		if allLow[i] < MinValidVoltage {
			return errors.New("voltage on L is lower than minimum valid")
		}

		if allHigh[i] > MaxValidVoltage {
			return errors.New("voltage on H is higher than maximum valid")
		}
	}
	return nil
}

// Simulate wired-AND behavior by deriving the bus voltage levels from all connected transceivers
func doWiredAND(this *component.Component, allLow, allHigh []physical.Voltage) error {
	// For simplicity, we approximate this by using min(CAN_L) and max(CAN_H) across all nodes
	busLow := slices.Min(allLow)
	busHigh := slices.Max(allHigh)

	this.Logger().Printf("bus voltage is L:%v / H:%v", busLow, busHigh)

	this.OutputByName(common.PortCANL).PutSignals(signal.New(busLow))
	this.OutputByName(common.PortCANH).PutSignals(signal.New(busHigh))
	return nil
}
