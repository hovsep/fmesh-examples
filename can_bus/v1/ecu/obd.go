package ecu

import (
	"github.com/hovsep/fmesh-examples/can_bus/v1/can"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
)

const (
	PortOBDIn   = "obd_in"
	PortOBDOut  = "obd_out"
	OBDUnitName = "obd"
	OBDNodeID   = 0x7DF
)

func NewOBD() *can.Node {
	obdDevice := can.NewNode(OBDUnitName, func(state component.State) {
		state.Set(ecuMemCanID, OBDNodeID)
	},
		func(this *component.Component) error {
			// Everything received by OBD interface goes to can bus
			return port.ForwardSignals(this.InputByName(PortOBDIn), this.OutputByName(can.PortCANTx))
		})

	// Add custom ports
	obdDevice.MCU.
		// Physical 16 pin OBD socket (io)
		WithInputs(PortOBDIn).
		WithOutputs(PortOBDOut)

	return obdDevice
}
