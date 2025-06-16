package ecu

import (
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/common"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
)

const (
	PortOBDIn   = "obd_in"
	PortOBDOut  = "obd_out"
	OBDUnitName = "obd"
)

// NewOBD creates a OBD can node
// in real life OBD socket is not a can node, but for simplicity
// we simulate OBD socket with plugged in OBD adapter as a single CAN node
func NewOBD() *can.Node {
	obdDevice := can.NewNode(OBDUnitName, func(state component.State) {
	},
		func(this *component.Component) error {
			// Everything received by OBD interface goes to can bus (todo: make it realistic, process only first signal, as OBD can not receive multiple frames at the same time)
			return port.ForwardSignals(this.InputByName(PortOBDIn), this.OutputByName(common.PortCANTx))
		})

	// Add custom ports
	obdDevice.MCU.
		// Physical 16 pin OBD socket (io)
		WithInputs(PortOBDIn).
		WithOutputs(PortOBDOut)

	return obdDevice
}
