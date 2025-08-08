package main

import (
	"fmt"
	"os"

	"github.com/hovsep/fmesh-examples/can_bus/advanced/diagnostics"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/ecu/engine"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/ecu/obd"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/ecu/transmission"

	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can/bus"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can"
)

func main() {
	// Create components:
	ptBus := bus.New("PT-CAN")                            // Modern vehicles have multiple buses, this one is called "powertrain bus"
	laptop := diagnostics.NewLaptop("lenovo-ideapad-340") // Laptop running diagnostic software and connected to vehicle via OBD socket

	// Build CAN nodes:
	obdDevice := obd.NewNode() // putting this into a variable, so we can connect it to the laptop
	allCanNodes := can.Nodes{
		engine.NewNode(),       // Engine Control Module
		transmission.NewNode(), // Transmission Control Module
		obdDevice,              // On Board Diagnostics
	}

	allCanNodes.ConnectToBus(ptBus)

	// Connect laptop to OBD socket
	err := laptop.ConnectToOBD(obdDevice)
	if err != nil {
		panic("Failed to connect laptop to OBD: " + err.Error())
	}

	// Build the mesh
	fm := fmesh.NewWithConfig("can_bus_sim_v1", &fmesh.Config{
		ErrorHandlingStrategy: fmesh.StopOnFirstErrorOrPanic,
		Debug:                 false,
	}).
		WithComponents(laptop.GetAllComponents()...).
		WithComponents(ptBus.GetAllComponents()...).
		WithComponents(allCanNodes.GetAllComponents()...)

	// Initialize the mesh:

	// set diagnostic frames to USB port, so the laptop will send them
	laptop.SendDataToUSB(
		diagnostics.FrameGetEngineDTCs,
		diagnostics.FrameGetSpeed,
		diagnostics.FrameGetRPM,
		diagnostics.FrameGetCoolantTemp,
		diagnostics.FrameGetCalibrationID,
		diagnostics.FrameGetVIN)

	runResult, err := fm.Run()
	if err != nil {
		fmt.Println("The mesh finished with error: ", err)
		os.Exit(1)
	}

	fmt.Printf("Mesh stopped after %d cycles and %s", runResult.Cycles.Len(), runResult.Duration)
}
