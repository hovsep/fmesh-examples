package main

import (
	"fmt"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/can_bus/v1/can"
	"github.com/hovsep/fmesh-examples/can_bus/v1/ecu"
	"github.com/hovsep/fmesh-graphviz/dot"
)

func main() {
	// Create components:
	bus := can.NewBus("PT-CAN")     // Modern vehicles have multiple buses, this one is called "powertrain bus"
	laptop := NewComputer("laptop") // Laptop running diagnostic software and connected to vehicle via OBD socket

	// Build CAN nodes:
	obdDevice := ecu.NewOBD() // put this into variable, so we can connect it to laptop
	allCanNodes := can.Nodes{
		ecu.NewECM(), // Engine Control Module
		ecu.NewTCM(), // Transmission Control Module
		ecu.NewACU(), // Airbag Control Unit
		obdDevice,    // On Board Diagnostics
	}

	allCanNodes.ConnectToBus(bus)

	// Connect usb-obd cable:
	laptop.OutputByName(portUSBOut).PipeTo(obdDevice.MCU.InputByName(ecu.PortOBDIn))
	obdDevice.MCU.OutputByName(ecu.PortOBDOut).PipeTo(laptop.InputByName(portUSBIn))

	// Build the mesh
	fm := fmesh.New("can_bus_sim_v1").
		WithComponents(bus, laptop).
		WithComponents(allCanNodes.GetAllComponents()...)

	// Send initial frames
	sendPayloadToUSBPort(laptop, frameDiagnosticRequest)

	// TODO: remove after debugged
	exporter := dot.NewDotExporter()
	data, _ := exporter.Export(fm)
	_ = data

	runResult, err := fm.Run()
	if err != nil {
		panic("Error running mesh: " + err.Error())
	}

	fmt.Printf("Mesh stopped after %d cycles", runResult.Cycles.Len())
}
