package main

import (
	"fmt"
	"os"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/bus"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/diagnostics"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/ecu/engine"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/ecu/obd"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/ecu/transmission"
	"github.com/hovsep/fmesh-examples/internal"
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
//   1. We inject diagnostic frames into the laptop's programmatic port.
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

var laptopInstance *diagnostics.Laptop

func main() {
	fm, err := getMesh()
	if err != nil {
		fmt.Println("Failed to build mesh:", err)
		os.Exit(1)
	}

	// Generate graphs if needed
	err = internal.HandleGraphFlag(fm, true)
	if err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}

	// Initialize the mesh: set diagnostic frames to USB port, so the laptop will send them
	laptopInstance.SendDataToUSB(
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
		fmt.Println("The mesh finished with error:", err)
		os.Exit(1)
	}

	fmt.Printf("Mesh stopped after %d cycles and %s", runResult.Cycles.Len(), runResult.Duration())
}

func getMesh() (*fmesh.FMesh, error) {
	// Create components:
	ptBus, err := bus.New("PT-CAN") // Modern vehicles have multiple buses, this one is called "powertrain bus"
	if err != nil {
		return nil, fmt.Errorf("bus: %w", err)
	}
	laptopInstance, err = diagnostics.NewLaptop("lenovo-ideapad-340") // Laptop running diagnostic software and connected to vehicle via OBD socket
	if err != nil {
		return nil, fmt.Errorf("laptop: %w", err)
	}

	// Build CAN nodes:
	obdDevice, err := obd.NewNode() // putting this into a variable, so we can connect it to the laptop
	if err != nil {
		return nil, fmt.Errorf("obd: %w", err)
	}
	engineNode, err := engine.NewNode()
	if err != nil {
		return nil, fmt.Errorf("engine: %w", err)
	}
	transmissionNode, err := transmission.NewNode()
	if err != nil {
		return nil, fmt.Errorf("transmission: %w", err)
	}
	allCanNodes := can.Nodes{
		engineNode,       // Engine Control Module
		transmissionNode, // Transmission Control Module
		obdDevice,        // On Board Diagnostics
	}

	if err := allCanNodes.ConnectToBus(ptBus); err != nil {
		return nil, fmt.Errorf("connect to bus: %w", err)
	}

	// Connect laptop to OBD socket
	err = laptopInstance.ConnectToOBD(obdDevice)
	if err != nil {
		return nil, fmt.Errorf("laptop→OBD: %w", err)
	}

	// Build the mesh
	fm, err := fmesh.New("can_bus_sim_v1",
		fmesh.WithErrorHandlingStrategy(fmesh.StopOnFirstErrorOrPanic), fmesh.WithUnlimitedCycles(),
	)
	if err != nil {
		return nil, fmt.Errorf("new mesh: %w", err)
	}

	if err := fm.AddComponents(laptopInstance.GetAllComponents()...); err != nil {
		return nil, fmt.Errorf("add laptop: %w", err)
	}
	if err := fm.AddComponents(ptBus.GetAllComponents()...); err != nil {
		return nil, fmt.Errorf("add bus: %w", err)
	}
	if err := fm.AddComponents(allCanNodes.GetAllComponents()...); err != nil {
		return nil, fmt.Errorf("add nodes: %w", err)
	}

	return fm, nil
}
