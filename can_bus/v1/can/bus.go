package can

import (
	"errors"
	"github.com/hovsep/fmesh/component"
	"slices"
)

const (
	MinValidVoltage = Voltage(1.0)
	MaxValidVoltage = Voltage(4.0)
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
			// TODO: add in-place noise generator

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

			// Basic validations
			if len(allLow) != len(allHigh) {
				return errors.New("voltages count mismatch")
			}

			// Detect faulty transceivers
			for i := 0; i < len(allLow); i++ {
				if allLow[i] > allHigh[i] {
					return errors.New("voltage on CAN_L is higher than on CAN_H")
				}

				if allLow[i] < MinValidVoltage {
					return errors.New("voltage on CAN_L is lower than minimum valid")
				}

				if allHigh[i] > MaxValidVoltage {
					return errors.New("voltage on CAN_H is higher than maximum valid")
				}
			}

			// Calculate bus voltage using min(CAN_L)\max(CAN_H) approximation
			busLow = slices.Min(allLow)
			busHigh = slices.Max(allHigh)

			this.Logger().Printf("bus voltage is now %v / %v", busLow, busHigh)

			return nil
		})
}
