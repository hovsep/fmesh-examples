package bus

import (
	"errors"
	"fmt"
	"slices"

	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/codec"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/common"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	MinValidVoltage = Voltage(0.5)
	MaxValidVoltage = Voltage(4.5)

	portInitialRecessiveSignals = "initial_recessive_signals"
)

// New creates a new CAN bus
func New(name string) *component.Component {
	bus := component.New("can_bus-"+name).
		WithInputs(common.PortCANL, common.PortCANH, portInitialRecessiveSignals).
		WithOutputs(common.PortCANL, common.PortCANH, portInitialRecessiveSignals).
		WithLogger(common.NewNoopLogger()).
		WithActivationFunc(func(this *component.Component) error {
			allLow, allHigh, err := processInitSignals(this)
			if err != nil {
				return fmt.Errorf("failed to process init signals: %w", err)
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
	bus.OutputByName(portInitialRecessiveSignals).PipeTo(bus.InputByName(portInitialRecessiveSignals))

	// Drive the bus with 11 recessive bits to simulate passive idle state,
	// ensuring all CAN controllers detect bus idle condition.
	bus.InputByName(portInitialRecessiveSignals).PutSignals(signal.New(codec.ProtocolEOFBitsCount + codec.ProtocolIFSBitsCount + 1))

	return bus
}

// Process init signals (the very first idle state, when bus is just powered)
func processInitSignals(this *component.Component) ([]Voltage, []Voltage, error) {
	var allLow, allHigh []Voltage

	initialRecessivesCount := this.InputByName(portInitialRecessiveSignals).FirstSignalPayloadOrDefault(0).(int)
	if initialRecessivesCount > 0 {
		allLow = append(allLow, RecessiveVoltage)
		allHigh = append(allHigh, RecessiveVoltage)

		// Self-activate
		if initialRecessivesCount > 1 {
			this.OutputByName(portInitialRecessiveSignals).PutSignals(signal.New(initialRecessivesCount - 1))
		}
	}
	return allLow, allHigh, nil
}

// Collect CAN_L voltages
func collectLow(this *component.Component, allLow []Voltage) ([]Voltage, error) {
	for _, sig := range this.InputByName(common.PortCANL).AllSignalsOrNil() {
		v, ok := sig.PayloadOrNil().(Voltage)
		if !ok {
			this.Logger().Println("bus received corrupted voltage on CAN_L wire")
		}

		allLow = append(allLow, v)
	}
	return allLow, nil
}

// Collect CAN_H voltages
func collectHigh(this *component.Component, allHigh []Voltage) ([]Voltage, error) {
	for _, sig := range this.InputByName(common.PortCANH).AllSignalsOrNil() {
		v, ok := sig.PayloadOrNil().(Voltage)
		if !ok {
			this.Logger().Println("bus received corrupted voltage on CAN_H wire")
		}

		allHigh = append(allHigh, v)
	}
	return allHigh, nil
}

func validateVoltages(allLow, allHigh []Voltage) error {
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
func doWiredAND(this *component.Component, allLow, allHigh []Voltage) error {
	// For simplicity, we approximate this by using min(CAN_L) and max(CAN_H) across all nodes
	busLow := slices.Min(allLow)
	busHigh := slices.Max(allHigh)

	this.Logger().Printf("bus voltage is L:%v / H:%v", busLow, busHigh)

	this.OutputByName(common.PortCANL).PutSignals(signal.New(busLow))
	this.OutputByName(common.PortCANH).PutSignals(signal.New(busHigh))
	return nil
}
