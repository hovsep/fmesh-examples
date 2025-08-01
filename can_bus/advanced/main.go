package main

import (
	"fmt"
	"os"

	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can/bus"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/ecu"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can"
)

func main() {
	// Create components:
	ptBus := bus.New("PT-CAN")      // Modern vehicles have multiple buses, this one is called "powertrain bus"
	laptop := NewComputer("laptop") // Laptop running diagnostic software and connected to vehicle via OBD socket

	// Build CAN nodes:
	obdDevice := ecu.NewOBD() // putting this into a variable, so we can connect it to the laptop
	allCanNodes := can.Nodes{
		ecu.NewECM(), // Engine Control Module
		ecu.NewTCM(), // Transmission Control Module
		ecu.NewACU(), // Airbag Control Unit
		obdDevice,    // On Board Diagnostics
	}

	allCanNodes.ConnectToBus(ptBus)

	// Connect usb-obd cable:
	laptop.OutputByName(portUSBOut).PipeTo(obdDevice.MCU.InputByName(ecu.PortOBDIn))
	obdDevice.MCU.OutputByName(ecu.PortOBDOut).PipeTo(laptop.InputByName(portUSBIn))

	// Build the mesh
	fm := fmesh.NewWithConfig("can_bus_sim_v1", &fmesh.Config{
		ErrorHandlingStrategy: fmesh.StopOnFirstErrorOrPanic,
		Debug:                 false,
	}).
		WithComponents(laptop).
		WithComponents(ptBus.GetAllComponents()...).
		WithComponents(allCanNodes.GetAllComponents()...)

	// Initialize the mesh:
	// send some data through laptop into OBD socket
	sendPayloadsToUSBPort(laptop,
		diagnosticFrameGetEngineDTCs,
		diagnosticFrameGetSpeed,
		diagnosticFrameGetRPM,
		diagnosticFrameGetCoolantTemp,
		diagnosticFrameGetCalibrationID,
		diagnosticFrameGetVIN)

	runResult, err := fm.Run()
	if err != nil {
		fmt.Println("The mesh finished with error: ", err)
		os.Exit(1)
	}

	fmt.Printf("Mesh stopped after %d cycles and %s", runResult.Cycles.Len(), runResult.Duration)
}
