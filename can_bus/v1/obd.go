package main

import (
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
)

const (
	portOBDIn  = "obd_in"
	portOBDOut = "obd_out"
	ecuOBD     = "obd"
)

func getOBD(bus *component.Component) *CanNode {
	obdDevice := getCanNode(ecuOBD, bus, func(state component.State) {
		state.Set(ecuMemCanID, 0x7DF)
	},
		func(this *component.Component) error {
			// Everything received by OBD interface goes to can bus (through transceiver)
			return port.ForwardSignals(this.InputByName(portOBDIn), this.OutputByName(portCANTx))
		})

	// Add custom ports
	obdDevice.MCU.
		// Physical 16 pin OBD socket (io)
		WithInputs(portOBDIn).
		WithOutputs(portOBDOut)

	return obdDevice
}
