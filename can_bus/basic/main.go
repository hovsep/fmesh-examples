package main

import (
	"fmt"
	"os"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/labels"
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
	fm := getMesh()

	// Generate graphs if needed
	err := internal.HandleGraphFlag(fm)
	if err != nil {
		fmt.Println("Failed to generate graph: ", err)
		os.Exit(1)
	}

	runCycle := 0
	// Initial signal group includes both valid and corrupted frames
	signal.NewGroup(
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
	).ForEach(func(sig *signal.Signal) error {
		fm.ComponentByName(componentBus).InputByName(portIn).PutSignals(sig)

		fm.Logger().Println("======================")
		fm.Logger().Printf("Run #%d", runCycle)
		fm.Logger().Println("======================")

		if _, err := fm.Run(); err != nil {
			panic("Error running mesh: " + err.Error())
		}

		time.Sleep(delayBetweenFrames)
		runCycle++
		return nil
	})

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
	fm := fmesh.New("can_bus_sim_v0").AddComponents(getBus())

	// Create and connect all CAN nodes to the bus
	for id, name := range canNodes {
		canNode := getNode(name, id)

		// Wire node output to bus input and vice versa (bidirectional communication)
		canNode.OutputByName(portOut).PipeTo(fm.ComponentByName(componentBus).InputByName(portIn))
		fm.ComponentByName(componentBus).OutputByName(portOut).PipeTo(canNode.InputByName(portIn))

		// Register node in the mesh
		fm.AddComponents(canNode)
	}

	return fm
}

// getBus returns a simple broadcasting CAN bus component
// All incoming signals are forwarded to all connected nodes
func getBus() *component.Component {
	return component.New(componentBus).
		AddInputs(portIn).
		AddOutputs(portOut).
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
		AddInputs(portIn).
		AddOutputs(portOut).
		WithActivationFunc(func(this *component.Component) error {
			myId := this.State().Get(stateNodeId).(int)
			validFrames := make([]CanFrame, 0)

			this.InputByName(portIn).Signals().ForEach(func(sig *signal.Signal) error {
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
							}).AddLabels(
							// Additionally, we can add some meta-data
							labels.Map{
								"from":       this.Name(),
								"detectedAt": time.Now().Format(time.RFC3339Nano),
							}),
					)
					return nil
				}

				// Ignore frames not addressed to this node
				if canFrame.Id != myId {
					this.Logger().Printf("Frame ID mismatch: expected %d, got %d", myId, canFrame.Id)
					return nil
				}

				validFrames = append(validFrames, canFrame)
				return nil
			})

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
