package main

import (
	"fmt"

	"github.com/hovsep/fmesh"
)

func main() {

	// Create needed components:
	canBus := getBus()
	laptop := getComputer("laptop")

	// Build CAN nodes:
	obdDevice := getOBD(canBus)
	allCanNodes := CanNodesSlice{
		getECM(canBus), // Engine Control Module
		getTCM(canBus), // Transmission Control Module
		getHU(canBus),  // Infotainment Head Unit
		getACU(canBus), // Airbag Control Unit
		obdDevice,      // On Board Diagnostics
	}

	// Connect usb-obd cable:
	laptop.OutputByName(portUSBOut).PipeTo(obdDevice.MCU.InputByName(portOBDIn))
	obdDevice.MCU.OutputByName(portOBDOut).PipeTo(laptop.InputByName(portUSBIn))

	// Build the mesh
	fm := fmesh.New("can_bus_sim_v1").
		WithComponents(canBus, laptop).
		WithComponents(allCanNodes.getAllComponents()...)

	// Send some signals into the system
	sendSignalToUSBPort(laptop, startEngine)

	runResult, err := fm.Run()
	if err != nil {
		panic("Error running mesh: " + err.Error())
	}

	fmt.Printf("Mesh stopped after %d cycles", runResult.Cycles.Len())
}
