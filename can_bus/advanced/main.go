package main

import (
	"fmt"
	"os"

	"github.com/hovsep/fmesh-example/can_bus/advanced/can"
	"github.com/hovsep/fmesh-example/can_bus/advanced/can/bus"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/diagnostics"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/ecu/engine"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/ecu/obd"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/ecu/transmission"

	"github.com/hovsep/fmesh"
)

// This demo simulates a CAN bus system with a laptop connected via a USB–OBD interface.
//
// The simulated bus has three connected nodes:
//   - Engine Control Unit (ECU)
//   - Transmission Control Unit (TCU)
//   - On-Board Diagnostics (OBD) socket
//
// Each node consists of:
//   - A Microcontroller Unit (MCU) running high-level application logic
//   - A CAN Controller handling CAN frame encoding/decoding at the protocol level
//   - A CAN Transceiver converting bits to physical voltage signals on the bus wires
//
// The bus itself consists of:
//   - A differential pair of wires (CAN_H, CAN_L) implementing wired-AND logic
//   - A "watchdog" component simulating the terminating resistors/transistors found on a physical bus
//
// The laptop:
//   - Has a USB interface connected to the OBD socket
//   - Includes a "programmatic" port for injecting data directly into the simulation
//
// Simulation flow:
//   1. We inject diagnostic frames into the laptop’s programmatic port.
//   2. The laptop forwards any "USB-labeled" frames to its USB port.
//   3. The USB connection routes data to the OBD socket.
//   4. The OBD node simply relays received data to the CAN bus, and forwards bus data to its output.
//   5. Once diagnostic frames reach the bus, all connected ECUs receive them.
//   6. The receive path in any node is: Transceiver (voltages) → Controller (bits) → MCU (frames).
//      The transmit path is the reverse.
//   7. MCUs may optionally run higher-layer protocols on top of CAN (e.g., ISO-TP).
//   8. Depending on the addressing mode (functional vs physical), requests may be answered by multiple ECUs (e.g., VIN request) or by a single ECU (e.g., gear position).
//
// Notes:
//   - This is a simplified model: CAN frames here omit CRC and ACK fields.
//   - However, essential behaviors such as bit stuffing, arbitration, and wired-AND logic are implemented.
//   - Powered by F-Mesh, all nodes run concurrently without explicitly using goroutines— even components within the same node can run in parallel.
//   - The architecture is modular: you can add more nodes, noise generators, or even virtual instruments (e.g., a voltmeter to plot bus waveforms).

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
		diagnostics.FrameGetCoolantTemperature,
		diagnostics.FrameGetCalibrationID,
		diagnostics.FrameGetVIN,
		diagnostics.FrameGetTransmissionFluidTemperature,
	)

	runResult, err := fm.Run()
	if err != nil {
		fmt.Println("The mesh finished with error: ", err)
		os.Exit(1)
	}

	fmt.Printf("Mesh stopped after %d cycles and %s", runResult.Cycles.Len(), runResult.Duration)
}
