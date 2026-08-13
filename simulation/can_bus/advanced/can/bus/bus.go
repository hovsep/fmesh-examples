package bus

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/simulation/can_bus/advanced/can/common"
	"github.com/hovsep/fmesh-examples/simulation/can_bus/advanced/can/physical"
	"github.com/hovsep/fmesh/component"
)

type Bus struct {
	Wires    *component.Component // Simulates the differential pair
	Watchdog *component.Component // Terminal resistors and halt logic
}

const (
	MinValidVoltage = physical.Voltage(0.5)
	MaxValidVoltage = physical.Voltage(4.5)

	portRecessiveBitRequest = "recessive_bit_request"
)

// New creates a new CAN bus
func New(name string) (*Bus, error) {
	wires, err := newWires(name + "-wires")
	if err != nil {
		return nil, fmt.Errorf("bus %s: %w", name, err)
	}
	watchDog, err := newWatchdog(name + "-watchdog")
	if err != nil {
		return nil, fmt.Errorf("bus %s: %w", name, err)
	}

	if err := wires.OutputByName(common.PortCANL).PipeTo(watchDog.InputByName(common.PortCANL)); err != nil {
		return nil, fmt.Errorf("bus %s: wire→watchdog CAN_L: %w", name, err)
	}
	if err := wires.OutputByName(common.PortCANH).PipeTo(watchDog.InputByName(common.PortCANH)); err != nil {
		return nil, fmt.Errorf("bus %s: wire→watchdog CAN_H: %w", name, err)
	}

	if err := watchDog.OutputByName(portRecessiveBitRequest).PipeTo(wires.InputByName(portRecessiveBitRequest)); err != nil {
		return nil, fmt.Errorf("bus %s: watchdog→wire: %w", name, err)
	}

	return &Bus{
		Wires:    wires,
		Watchdog: watchDog,
	}, nil
}

// GetAllComponents returns all fmesh components of the Bus
func (b Bus) GetAllComponents() []*component.Component {
	return []*component.Component{
		b.Wires,
		b.Watchdog,
	}
}
