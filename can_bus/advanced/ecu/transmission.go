package ecu

import (
	"github.com/hovsep/fmesh-examples/can_bus/advanced/diagnostics"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can"
)

const (
	TCMUnitName = "tcm"
)

var (
	P0710 = diagnostics.DTC{0x07, 0x10} // Fluid Temperature Sensor Circuit High
	P0705 = diagnostics.DTC{0x07, 0x05} // Range Sensor Circuit Malfunction
	P0740 = diagnostics.DTC{0x07, 0x40} // Control System Malfunction
)

func NewTCM() *can.Node {
	return can.NewNode(TCMUnitName, nil, nil)
}
