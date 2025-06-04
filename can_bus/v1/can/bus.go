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
)

// NewBus creates a new CAN bus
func NewBus(name string) *component.Component {
	return component.New("can_bus-"+name).
		WithInputs(PortCANL, PortCANH).
		WithOutputs(PortCANL, PortCANH).
		WithActivationFunc(func(this *component.Component) error {
			var (
				allLow  []Voltage
				allHigh []Voltage
			)

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

			// Simulate wired-AND logic by deriving the bus voltages from voltages on all connected transceivers
			// for simplicity we use min(CAN_L)\max(CAN_H) approximation
			busLow = slices.Min(allLow)
			busHigh = slices.Max(allHigh)

			this.Logger().Printf("bus voltage is L:%v / H:%v", busLow, busHigh)

			this.OutputByName(PortCANL).PutSignals(signal.New(busLow))
			this.OutputByName(PortCANH).PutSignals(signal.New(busHigh))
			return nil
		})
}
