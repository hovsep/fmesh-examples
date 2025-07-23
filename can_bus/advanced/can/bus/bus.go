package bus

import (
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/common"

	"github.com/hovsep/fmesh/component"
)

type Bus struct {
	Wires    *component.Component // Simulates the differential pair
	Watchdog *component.Component // Terminal resistors and halt logic
}

const (
	MinValidVoltage = Voltage(0.5)
	MaxValidVoltage = Voltage(4.5)

	portRecessiveBitRequest = "recessive_bit_request"
)

// New creates a new CAN bus
func New(name string) *Bus {
	wires := newWires(name + "-wires")
	watchDog := newWatchdog(name + "-watchdog")

	// wires -> watchdog
	wires.OutputByName(common.PortCANL).PipeTo(watchDog.InputByName(common.PortCANL))
	wires.OutputByName(common.PortCANH).PipeTo(watchDog.InputByName(common.PortCANH))

	// watchdog -> wires
	watchDog.OutputByName(portRecessiveBitRequest).PipeTo(wires.InputByName(portRecessiveBitRequest))

	return &Bus{
		Wires:    wires,
		Watchdog: watchDog,
	}
}

// GetAllComponents returns all fmesh components of the Bus
func (b Bus) GetAllComponents() []*component.Component {
	return []*component.Component{
		b.Wires,
		b.Watchdog,
	}
}
