package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh-examples/simulation/can_bus/advanced/can"
	"github.com/hovsep/fmesh-examples/simulation/can_bus/advanced/can/bus"
	"github.com/hovsep/fmesh-examples/simulation/can_bus/advanced/diagnostics"
	"github.com/hovsep/fmesh-examples/simulation/can_bus/advanced/ecu/engine"
	"github.com/hovsep/fmesh-examples/simulation/can_bus/advanced/ecu/obd"
	"github.com/hovsep/fmesh-examples/simulation/can_bus/advanced/ecu/transmission"
)

var laptopInstance *diagnostics.Laptop

func main() {
	fm, err := getMesh()
	if err != nil {
		fmt.Println("Failed to build mesh:", err)
		os.Exit(1)
	}

	err = internal.HandleGraphFlag(fm, true)
	if err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}

	fmt.Println("============================================")
	fmt.Println("  CAN Bus Advanced Simulation (Layered)")
	fmt.Println("============================================")
	fmt.Println("Architecture:")
	fmt.Println("  Bus: PT-CAN (powertrain) with differential wires (wired-AND)")
	fmt.Println("       + watchdog component (termination)")
	fmt.Println("  Each CAN node has 3 layers:")
	fmt.Println("    MCU (application logic)")
	fmt.Println("      → Controller (frame encoding, bit stuffing, arbitration)")
	fmt.Println("      → Transceiver (bit ↔ voltage on bus wires)")
	fmt.Println("  Nodes: Engine ECU, Transmission ECU, OBD socket")
	fmt.Println("  Laptop (Lenovo IdeaPad 340) → USB → OBD → CAN Bus")
	fmt.Println()
	fmt.Println("Sending diagnostic requests from laptop via OBD-II...")
	fmt.Println()

	laptopInstance.SendDataToUSB(
		diagnostics.FrameGetEngineDTCs,
		diagnostics.FrameGetSpeed,
		diagnostics.FrameGetRPM,
		diagnostics.FrameGetCoolantTemperature,
		diagnostics.FrameGetCalibrationID,
		diagnostics.FrameGetVIN,
		diagnostics.FrameGetTransmissionFluidTemperature,
	)

	fmt.Println("Path: Laptop → USB → OBD Socket → CAN Bus → ECUs")
	fmt.Println("ECUs with matching IDs will process and respond.")
	fmt.Println()
	fmt.Println("Running simulation...")
	fmt.Println()

	runResult, err := fm.Run(context.Background())
	if err != nil {
		fmt.Println("The mesh finished with error:", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("============================================")
	fmt.Println("  Simulation Complete")
	fmt.Println("============================================")
	fmt.Printf("  Mesh ran %d cycles in %s\n", runResult.Cycles.Len(), runResult.Duration())
	fmt.Println("  Diagnostic requests routed: Laptop → USB → OBD → CAN Bus → ECUs")
	fmt.Println("  ECUs processed matching requests; responses flowed back through the bus.")
	fmt.Println()
}

func getMesh() (*fmesh.FMesh, error) {
	ptBus, err := bus.New("PT-CAN")
	if err != nil {
		return nil, fmt.Errorf("bus: %w", err)
	}
	laptopInstance, err = diagnostics.NewLaptop("lenovo-ideapad-340")
	if err != nil {
		return nil, fmt.Errorf("laptop: %w", err)
	}

	obdDevice, err := obd.NewNode()
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
		engineNode,
		transmissionNode,
		obdDevice,
	}

	if err := allCanNodes.ConnectToBus(ptBus); err != nil {
		return nil, fmt.Errorf("connect to bus: %w", err)
	}

	err = laptopInstance.ConnectToOBD(obdDevice)
	if err != nil {
		return nil, fmt.Errorf("laptop→OBD: %w", err)
	}

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
