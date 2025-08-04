package ecu

import (
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/microcontroller"
)

const (
	TCMUnitName = "tcm"
)

var (
	P0710 = microcontroller.DTC{0x07, 0x10} // Fluid Temperature Sensor Circuit High
	P0705 = microcontroller.DTC{0x07, 0x05} // Range Sensor Circuit Malfunction
	P0740 = microcontroller.DTC{0x07, 0x40} // Control System Malfunction
)

func NewTCM() *can.Node {
	return can.NewNode(TCMUnitName, nil, nil)
}
