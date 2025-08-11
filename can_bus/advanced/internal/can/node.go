package can

import (
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can/bus"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can/common"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can/controller"
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/microcontroller"
	"github.com/hovsep/fmesh/component"
)

// The Node consists of multiple components
type Node struct {
	MCU         *component.Component // Main logic, operates with ISO-TP messages
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
	mcu.OutputByName(common.PortCANTx).PipeTo(ctl.InputByName(common.PortCANTx))
	// mcu <- controller
	ctl.OutputByName(common.PortCANRx).PipeTo(mcu.InputByName(common.PortCANRx))

	// controller -> transceiver
	ctl.OutputByName(common.PortCANTx).PipeTo(trsv.InputByName(common.PortCANTx))
	// controller <- transceiver
	trsv.OutputByName(common.PortCANRx).PipeTo(ctl.InputByName(common.PortCANRx))

	return &Node{
		MCU:         mcu,
		Controller:  ctl,
		Transceiver: trsv,
	}
}

// GetAllComponents returns all fmesh components of node
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
		node.Transceiver.OutputByName(common.PortCANL).PipeTo(b.Wires.InputByName(common.PortCANL))
		node.Transceiver.OutputByName(common.PortCANH).PipeTo(b.Wires.InputByName(common.PortCANH))

		// transceiver <- bus:
		b.Wires.OutputByName(common.PortCANL).PipeTo(node.Transceiver.InputByName(common.PortCANL))
		b.Wires.OutputByName(common.PortCANH).PipeTo(node.Transceiver.InputByName(common.PortCANH))

		// ctl -> bus watchdog
		node.Controller.OutputByName(common.PortControllerState).PipeTo(b.Watchdog.InputByName(common.PortControllerState))

	}
}
