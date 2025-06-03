package main

import (
	"fmt"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/can_bus/v1/can"
	"github.com/hovsep/fmesh-examples/can_bus/v1/ecu"
)

func main() {
	// Create components:
	canBus := can.NewBus("main")
	laptop := NewComputer("laptop")

	// Build CAN nodes:
	obdDevice := ecu.NewOBD(canBus) // put this into variable, so we can connect it to laptop
	allCanNodes := can.Nodes{
		ecu.NewECM(canBus), // Engine Control Module
		ecu.NewTCM(canBus), // Transmission Control Module
		ecu.NewHU(canBus),  // Infotainment Head Unit
		ecu.NewACU(canBus), // Airbag Control Unit
		obdDevice,          // On Board Diagnostics
	}

	// Connect usb-obd cable:
	laptop.OutputByName(portUSBOut).PipeTo(obdDevice.MCU.InputByName(ecu.PortOBDIn))
	obdDevice.MCU.OutputByName(ecu.PortOBDOut).PipeTo(laptop.InputByName(portUSBIn))

	// Build the mesh
	fm := fmesh.New("can_bus_sim_v1").
		WithComponents(canBus, laptop).
		WithComponents(allCanNodes.GetAllComponents()...)

	// Send initial frames
	sendPayloadToUSBPort(laptop, frameStartEngine)

	runResult, err := fm.Run()
	if err != nil {
		panic("Error running mesh: " + err.Error())
	}

	fmt.Printf("Mesh stopped after %d cycles", runResult.Cycles.Len())
}
