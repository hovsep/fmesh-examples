package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

const (
	portIn             = "rx"
	portOut            = "tx"
	stateNodeId        = "id"
	componentBus       = "bus"
	delayBetweenFrames = 1000 * time.Millisecond
)

type CanFrame struct {
	Id   int
	Data []byte
}

func main() {
	fm, err := getMesh()
	if err != nil {
		fmt.Println("Failed to build mesh:", err)
		os.Exit(1)
	}

	fmt.Println("========================================")
	fmt.Println("  CAN Bus Basic Simulation")
	fmt.Println("========================================")
	fmt.Println("Architecture:")
	fmt.Println("  1 bus component forwards every frame to all connected ECUs")
	fmt.Println("  Each ECU processes only frames matching its own ID")
	fmt.Println("  Invalid payloads are reported as corrupted signals")
	fmt.Println()

	if err := internal.HandleGraphFlag(fm, true); err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}

	runCycle := 0

	frames := signal.NewGroup(
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
	fmt.Printf("Injecting %d CAN frames into the bus...\n", frames.Len())
	fmt.Println()

	frames.ForEach(func(sig *signal.Signal) error {
		fm.ComponentByName(componentBus).InputByName(portIn).PutSignals(sig)

		fm.Logger().Println("======================")
		fm.Logger().Printf("Run #%d", runCycle)
		fm.Logger().Println("======================")

		if _, err := fm.Run(context.Background()); err != nil {
			fmt.Println("Error running mesh:", err)
			os.Exit(1)
		}

		time.Sleep(delayBetweenFrames)
		runCycle++
		return nil
	})

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("  Simulation Complete")
	fmt.Println("========================================")
	fmt.Println()
}

func getMesh() (*fmesh.FMesh, error) {
	canNodes := []string{
		"engine-ecu",
		"airbag-ecu",
		"crash-sensor-front-left",
		"door-lock-actuator-rear-left",
		"obd",
	}

	fm, err := fmesh.New("can_bus_sim_v0",
		fmesh.WithErrorHandlingStrategy(fmesh.StopOnFirstErrorOrPanic),
	)
	if err != nil {
		return nil, fmt.Errorf("new mesh: %w", err)
	}

	bus, err := getBus()
	if err != nil {
		return nil, fmt.Errorf("bus: %w", err)
	}
	if err := fm.AddComponents(bus); err != nil {
		return nil, fmt.Errorf("add bus: %w", err)
	}

	for id, name := range canNodes {
		canNode, err := getNode(name, id)
		if err != nil {
			return nil, fmt.Errorf("node %s: %w", name, err)
		}

		if err := canNode.OutputByName(portOut).PipeTo(fm.ComponentByName(componentBus).InputByName(portIn)); err != nil {
			return nil, fmt.Errorf("pipe %s→bus: %w", name, err)
		}
		if err := fm.ComponentByName(componentBus).OutputByName(portOut).PipeTo(canNode.InputByName(portIn)); err != nil {
			return nil, fmt.Errorf("pipe bus→%s: %w", name, err)
		}
		if err := fm.AddComponents(canNode); err != nil {
			return nil, fmt.Errorf("add %s: %w", name, err)
		}
	}

	return fm, nil
}

func getBus() (*component.Component, error) {
	return component.New(componentBus,
		component.WithInputs(portIn),
		component.WithOutputs(portOut),
		component.WithActivationFunc(func(ctx context.Context, this *component.Component) error {
			return port.ForwardSignals(ctx, this.InputByName(portIn), this.OutputByName(portOut))
		}),
	)
}

func getNode(name string, id int) (*component.Component, error) {
	return component.New(name,
		component.WithInitialState(func(state component.State) {
			state.Set(stateNodeId, id)
		}),
		component.WithInputs(portIn),
		component.WithOutputs(portOut),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			myId := this.State().Get(stateNodeId).(int)
			validFrames := make([]CanFrame, 0)

			this.InputByName(portIn).Signals().ForEach(func(sig *signal.Signal) error {
				canFrame, ok := sig.Payload().(CanFrame)
				if !ok {
					this.Logger().Printf("Invalid frame received, skipping: %v", sig.Payload())
					this.OutputByName(portOut).PutSignals(
						signal.New(
							CanFrame{
								Id:   4,
								Data: fmt.Appendf(nil, "register corrupted singal: %v", sig.Payload()),
							}).WithLabels(
							map[string]string{
								"from":       this.Name(),
								"detectedAt": time.Now().Format(time.RFC3339Nano),
							}),
					)
					return nil
				}

				if canFrame.Id != myId {
					this.Logger().Printf("Frame ID mismatch: expected %d, got %d", myId, canFrame.Id)
					return nil
				}

				validFrames = append(validFrames, canFrame)
				return nil
			})

			if len(validFrames) == 0 {
				return nil
			}

			for _, frame := range validFrames {
				this.Logger().Printf("Processing data: %s", frame.Data)
			}

			return nil
		}),
	)
}
