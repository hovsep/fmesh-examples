package can

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/bus"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/common"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can/controller"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/microcontroller"
	"github.com/hovsep/fmesh/component"
)

// The Node consists of multiple components
type Node struct {
	MCU         *component.Component // Main logic operates with ISO-TP messages
	Controller  *component.Component // Converts frames to bits and vice versa
	Transceiver *component.Component // Converts bits to voltages
}

// Nodes hold multiple nodes without any guarantees of order
type Nodes []*Node

// NewNode creates a new CAN node
func NewNode(unitName string, mcuInitState func(state component.State), mcuActivationFunction component.ActivationFunc) *Node {
	// Create electronic components
	mcu := microcontroller.New(unitName, mcuInitState, mcuActivationFunction)
	ctl := controller.New(unitName)
	trsv := NewTransceiver(unitName)

	// Wiring : mcu <--> controller <--> transceiver

	// mcu -> controller:
	if err := mcu.OutputByName(common.PortCANTx).PipeTo(ctl.InputByName(common.PortCANTx)); err != nil {
		panic(fmt.Sprintf("failed to pipe mcu to ctl: %v", err))
	}
	// mcu <- controller
	if err := ctl.OutputByName(common.PortCANRx).PipeTo(mcu.InputByName(common.PortCANRx)); err != nil {
		panic(fmt.Sprintf("failed to pipe ctl to mcu: %v", err))
	}

	// controller -> transceiver
	if err := ctl.OutputByName(common.PortCANTx).PipeTo(trsv.InputByName(common.PortCANTx)); err != nil {
		panic(fmt.Sprintf("failed to pipe ctl to trsv: %v", err))
	}
	// controller <- transceiver
	if err := trsv.OutputByName(common.PortCANRx).PipeTo(ctl.InputByName(common.PortCANRx)); err != nil {
		panic(fmt.Sprintf("failed to pipe trsv to ctl: %v", err))
	}

	return &Node{
		MCU:         mcu,
		Controller:  ctl,
		Transceiver: trsv,
	}
}

// GetAllComponents returns all fmesh components of the node
func (nodes Nodes) GetAllComponents() []*component.Component {
	var all []*component.Component
	for _, node := range nodes {
		all = append(all, node.MCU, node.Controller, node.Transceiver)
	}
	return all
}

// ConnectToBus connect all nodes to the given bus
func (nodes Nodes) ConnectToBus(b *bus.Bus) {
	for _, node := range nodes {
		// transceiver -> bus:
		if err := node.Transceiver.OutputByName(common.PortCANL).PipeTo(b.Wires.InputByName(common.PortCANL)); err != nil {
			panic(fmt.Sprintf("failed to pipe transceiver to bus: %v", err))
		}
		if err := node.Transceiver.OutputByName(common.PortCANH).PipeTo(b.Wires.InputByName(common.PortCANH)); err != nil {
			panic(fmt.Sprintf("failed to pipe transceiver to bus: %v", err))
		}

		// transceiver <- bus:
		if err := b.Wires.OutputByName(common.PortCANL).PipeTo(node.Transceiver.InputByName(common.PortCANL)); err != nil {
			panic(fmt.Sprintf("failed to pipe bus to transceiver: %v", err))
		}
		if err := b.Wires.OutputByName(common.PortCANH).PipeTo(node.Transceiver.InputByName(common.PortCANH)); err != nil {
			panic(fmt.Sprintf("failed to pipe bus to transceiver: %v", err))
		}

		// ctl -> bus watchdog
		if err := node.Controller.OutputByName(common.PortControllerState).PipeTo(b.Watchdog.InputByName(common.PortControllerState)); err != nil {
			panic(fmt.Sprintf("failed to pipe ctl to watchdog: %v", err))
		}

	}
}
