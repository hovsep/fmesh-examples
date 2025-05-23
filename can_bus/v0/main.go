package main

import (
	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
	"time"
)

const (
	portIn       = "rx"
	portOut      = "tx"
	stateNodeId  = "id"
	componentBus = "bus"
)

// CanFrame represents simplistic abstraction of real CAN frame
type CanFrame struct {
	Id   int
	Data []byte
}

func main() {
	canNodes := []string{"engine-ecu", "airbag-ecu", "crash-sensor-front-left", "door-lock-actuator-rear-left"}

	// Create mesh
	fm := fmesh.New("can_bus_sim").WithComponents(getBus())

	// Create nodes
	for id, name := range canNodes {
		canNode := getNode(name, id)

		// Wiring
		canNode.OutputByName(portOut).PipeTo(fm.ComponentByName(componentBus).InputByName(portIn))
		fm.ComponentByName(componentBus).OutputByName(portOut).PipeTo(canNode.InputByName(portIn))

		// Add node to mesh
		fm.WithComponents(canNode)
	}

	initSignals := signal.NewGroup().WithPayloads(
		CanFrame{Id: 0, Data: []byte("ignition-start")},
		CanFrame{Id: 1, Data: []byte("deploy-airbag")},
		CanFrame{Id: 2, Data: []byte("impact-detected-front-left")},
		CanFrame{Id: 3, Data: []byte("lock-door")},
		"invalid signal 1",
		CanFrame{Id: 3, Data: []byte("unlock-door")},
		CanFrame{Id: 0, Data: []byte("rpm-update:3500")},
		"invalid signal 2",
		CanFrame{Id: 0, Data: []byte("engine-temp:90C")},
		CanFrame{Id: 1, Data: []byte("airbag-status:ok")},
		CanFrame{Id: 2, Data: []byte("sensor-selfcheck:pass")},
		CanFrame{Id: 3, Data: []byte("lock-status:locked")},
	)

	// Send frames
	for runCycle, sig := range initSignals.SignalsOrNil() {
		fm.ComponentByName(componentBus).InputByName(portIn).PutSignals(sig)
		fm.Logger().Println("======================")
		fm.Logger().Printf("Run #%d", runCycle)
		fm.Logger().Println("======================")
		_, err := fm.Run()
		if err != nil {
			panic("Error running mesh: " + err.Error())
		}
		time.Sleep(1 * time.Second)
	}

	fm.Logger().Println("======================")
	fm.Logger().Println("Simulation finished successfully")

}

// Returns simple CAN bus
func getBus() *component.Component {
	return component.New(componentBus).
		WithInputs(portIn).
		WithOutputs(portOut).
		WithActivationFunc(func(this *component.Component) error {
			// Simple broadcast
			return port.ForwardSignals(this.InputByName(portIn), this.OutputByName(portOut))
		})
}

// Returns simple can node
func getNode(name string, id int) *component.Component {
	return component.New(name + "-can_node").
		WithInitialState(func(state component.State) {
			state.Set(stateNodeId, id)
		}).
		WithInputs(portIn).
		WithOutputs(portOut).
		WithActivationFunc(func(this *component.Component) error {

			myId := this.State().Get(stateNodeId).(int)

			// Validation
			validFrames := make([]CanFrame, 0)
			for _, sig := range this.InputByName(portIn).AllSignalsOrNil() {
				canFrame, ok := sig.PayloadOrNil().(CanFrame)
				if !ok {
					this.Logger().Printf("Invalid frame received, skipping : %v", sig.PayloadOrNil())
					continue
				}

				if canFrame.Id != myId {
					this.Logger().Printf("Frame is not for me, id mismatch: %d != %d", myId, canFrame.Id)
					continue
				}

				validFrames = append(validFrames, canFrame)
			}

			if len(validFrames) == 0 {
				// No frames for me
				return nil
			}

			// Processing
			for _, frame := range validFrames {
				this.Logger().Printf("Processing data: %s", frame.Data)
			}

			return nil
		})
}
