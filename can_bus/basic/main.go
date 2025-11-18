package main

import (
	"fmt"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/tools/example-helper"
	"github.com/hovsep/fmesh/common"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

const (
	portIn             = "rx"
	portOut            = "tx"
	stateNodeId        = "id"
	componentBus       = "bus"
	delayBetweenFrames = 1000 * time.Millisecond // Delay between sending frames, for readability
)

// CanFrame represents a simplified CAN frame structure.
// In real systems, this would include control flags, length, CRC, etc.
type CanFrame struct {
	Id   int
	Data []byte
}

// This example demonstrates a minimal CAN bus simulation using F-Mesh
// A central bus component broadcasts all signals to connected CAN nodes
// Each node filters and processes only frames that match its assigned ID
// Features: signal broadcasting, ID-based filtering, and corruption detection
func main() {
	// Handle flags (--graph, etc.)
	if examplehelper.RunWithFlags(getMesh) {
		return // Exit if graph was generated
	}

	// Normal execution
	fm := getMesh()

	// Initial signal group includes both valid and corrupted frames
	initSignals := signal.NewGroup().WithPayloads(
		CanFrame{Id: 0, Data: []byte("ignition-start")},
		CanFrame{Id: 2, Data: []byte("impact-detected-front-left")},
		CanFrame{Id: 1, Data: []byte("deploy-airbag")},
		CanFrame{Id: 3, Data: []byte("lock-door")},
		"electrical noise",
		CanFrame{Id: 3, Data: []byte("unlock-door")},
		CanFrame{Id: 0, Data: []byte("rpm-update:3500")},
		"faulty transceiver",
		CanFrame{Id: 0, Data: []byte("engine-temp:90C")},
		CanFrame{Id: 1, Data: []byte("airbag-status:ok")},
		CanFrame{Id: 2, Data: []byte("sensor-selfcheck:pass")},
		CanFrame{Id: 3, Data: []byte("lock-status:locked")},
	)

	// Feed signals one by one into the bus for processing
	for runCycle, sig := range initSignals.SignalsOrNil() {
		fm.ComponentByName(componentBus).InputByName(portIn).PutSignals(sig)

		fm.Logger().Println("======================")
		fm.Logger().Printf("Run #%d", runCycle)
		fm.Logger().Println("======================")

		if _, err := fm.Run(); err != nil {
			panic("Error running mesh: " + err.Error())
		}

		time.Sleep(delayBetweenFrames)
	}

	fm.Logger().Println("======================")
	fm.Logger().Println("Simulation completed successfully.")
}

func getMesh() *fmesh.FMesh {
	// List of simulated CAN nodes. Order determines their ID (starting from 0)
	canNodes := []string{
		"engine-ecu",
		"airbag-ecu",
		"crash-sensor-front-left",
		"door-lock-actuator-rear-left",
		"obd", // On-Board Diagnostic Module
	}

	// Initialize the mesh with the central CAN bus component
	fm := fmesh.New("can_bus_sim_v0").WithComponents(getBus())

	// Create and connect all CAN nodes to the bus
	for id, name := range canNodes {
		canNode := getNode(name, id)

		// Wire node output to bus input and vice versa (bidirectional communication)
		canNode.OutputByName(portOut).PipeTo(fm.ComponentByName(componentBus).InputByName(portIn))
		fm.ComponentByName(componentBus).OutputByName(portOut).PipeTo(canNode.InputByName(portIn))

		// Register node in the mesh
		fm.WithComponents(canNode)
	}

	return fm
}

// getBus returns a simple broadcasting CAN bus component
// All incoming signals are forwarded to all connected nodes
func getBus() *component.Component {
	return component.New(componentBus).
		WithInputs(portIn).
		WithOutputs(portOut).
		WithActivationFunc(func(this *component.Component) error {
			return port.ForwardSignals(this.InputByName(portIn), this.OutputByName(portOut))
		})
}

// getNode returns a CAN node component
// Each node processes only frames with matching ID and logs the data
func getNode(name string, id int) *component.Component {
	return component.New(name).
		WithInitialState(func(state component.State) {
			state.Set(stateNodeId, id)
		}).
		WithInputs(portIn).
		WithOutputs(portOut).
		WithActivationFunc(func(this *component.Component) error {
			myId := this.State().Get(stateNodeId).(int)
			validFrames := make([]CanFrame, 0)

			for _, sig := range this.InputByName(portIn).AllSignalsOrNil() {
				// Reject corrupted signals
				canFrame, ok := sig.PayloadOrNil().(CanFrame)
				if !ok {
					this.Logger().Printf("Invalid frame received, skipping: %v", sig.PayloadOrNil())

					// Send it to OBD
					this.OutputByName(portOut).PutSignals(
						signal.New(
							CanFrame{
								Id:   4,
								Data: []byte(fmt.Sprintf("register corrupted singal: %v", sig.PayloadOrNil())),
							}).WithLabels(
							// Additionally we can add some meta-data
							common.LabelsCollection{
								"from":       this.Name(),
								"detectedAt": time.Now().Format(time.RFC3339Nano),
							}))
					continue
				}

				// Ignore frames not addressed to this node
				if canFrame.Id != myId {
					this.Logger().Printf("Frame ID mismatch: expected %d, got %d", myId, canFrame.Id)
					continue
				}

				validFrames = append(validFrames, canFrame)
			}

			if len(validFrames) == 0 {
				// No frames to process. Aborting activation
				return nil
			}

			// Process all valid frames
			for _, frame := range validFrames {
				this.Logger().Printf("Processing data: %s", frame.Data)
			}

			return nil
		})
}
