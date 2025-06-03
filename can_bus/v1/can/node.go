package can

import "github.com/hovsep/fmesh/component"

// Node consists of multiple components
type Node struct {
	MCU         *component.Component // Main logic, operates with frames
	Controller  *component.Component // Converts frames to bits and vice versa
	Transceiver *component.Component // Converts bits to voltages
}

// Nodes holds multiple nodes without any guarantees of order
type Nodes []*Node

// NewNode creates a new CAN node
func NewNode(unitName string, bus *component.Component, initState func(state component.State), mcuLogic component.ActivationFunc) *Node {
	// Create electronic components
	mcu := NewMCU(unitName, initState, mcuLogic)
	controller := NewController(unitName)
	transceiver := NewTransceiver(unitName)

	// Wiring : mcu <--> controller <--> transceiver <--> bus

	// mcu -> controller:
	mcu.OutputByName(PortCANTx).PipeTo(controller.InputByName(PortCANTx))
	// mcu <- controller
	controller.OutputByName(PortCANRx).PipeTo(mcu.InputByName(PortCANRx))

	// controller -> transceiver
	controller.OutputByName(PortCANTx).PipeTo(transceiver.InputByName(PortCANTx))
	// controller <- transceiver
	transceiver.OutputByName(PortCANRx).PipeTo(controller.InputByName(PortCANRx))

	// transceiver -> bus:
	transceiver.OutputByName(PortCANL).PipeTo(bus.InputByName(PortCANL))
	transceiver.OutputByName(PortCANH).PipeTo(bus.InputByName(PortCANH))
	// transceiver <- bus:
	bus.OutputByName(PortCANL).PipeTo(transceiver.InputByName(PortCANL))
	bus.OutputByName(PortCANH).PipeTo(transceiver.InputByName(PortCANH))

	return &Node{
		MCU:         mcu,
		Controller:  controller,
		Transceiver: transceiver,
	}
}

// GetAllComponents returns all fmesh components of which the group of nodes consists
func (nodes Nodes) GetAllComponents() []*component.Component {
	var all []*component.Component
	for _, ecu := range nodes {
		all = append(all, ecu.MCU, ecu.Controller, ecu.Transceiver)
	}
	return all
}
