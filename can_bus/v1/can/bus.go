package can

import (
	"errors"
	"slices"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	MinValidVoltage = Voltage(0.5)
	MaxValidVoltage = Voltage(4.5)

	portInitialRecessiveSignals = "initial_recessive_signals"
)

// NewBus creates a new CAN bus
func NewBus(name string) *component.Component {
	bus := component.New("can_bus-"+name).
		WithInputs(PortCANL, PortCANH, portInitialRecessiveSignals).
		WithOutputs(PortCANL, PortCANH, portInitialRecessiveSignals).
		WithActivationFunc(func(this *component.Component) error {
			var (
				allLow  []Voltage
				allHigh []Voltage
			)

			// Process init signals (the very first idle state)
			initialRecessivesCount := this.InputByName(portInitialRecessiveSignals).FirstSignalPayloadOrDefault(0).(int)
			if initialRecessivesCount > 0 {
				allLow = append(allLow, RecessiveVoltage)
				allHigh = append(allHigh, RecessiveVoltage)

				// Self-activate
				if initialRecessivesCount > 1 {
					this.OutputByName(portInitialRecessiveSignals).PutSignals(signal.New(initialRecessivesCount - 1))
				}
			}

			busLow, busHigh := RecessiveVoltage, RecessiveVoltage

			// Collect CAN_L voltages
			for _, sig := range this.InputByName(PortCANL).AllSignalsOrNil() {
				v, ok := sig.PayloadOrNil().(Voltage)
				if !ok {
					this.Logger().Println("bus received corrupted voltage on CAN_L wire")
				}

				allLow = append(allLow, v)
			}

			// Collect CAN_H voltages
			for _, sig := range this.InputByName(PortCANH).AllSignalsOrNil() {
				v, ok := sig.PayloadOrNil().(Voltage)
				if !ok {
					this.Logger().Println("bus received corrupted voltage on CAN_H wire")
				}

				allHigh = append(allHigh, v)
			}

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

			// Simulate wired-AND behavior by deriving the bus voltage levels from all connected transceivers
			// For simplicity, we approximate this by using min(CAN_L) and max(CAN_H) across all nodes
			busLow = slices.Min(allLow)
			busHigh = slices.Max(allHigh)

			this.Logger().Printf("bus voltage is L:%v / H:%v", busLow, busHigh)

			this.OutputByName(PortCANL).PutSignals(signal.New(busLow))
			this.OutputByName(PortCANH).PutSignals(signal.New(busHigh))
			return nil
		})

	// Setup self-activation pipe
	bus.OutputByName(portInitialRecessiveSignals).PipeTo(bus.InputByName(portInitialRecessiveSignals))

	// Drive the bus with 11 recessive bits to simulate passive idle state,
	// ensuring all CAN controllers detect bus idle condition.
	bus.InputByName(portInitialRecessiveSignals).PutSignals(signal.New(ProtocolEOFBitsCount + ProtocolIFSBitsCount + 1))

	return bus
}
