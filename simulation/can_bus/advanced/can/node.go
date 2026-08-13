package can

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/simulation/can_bus/advanced/can/bus"
	"github.com/hovsep/fmesh-examples/simulation/can_bus/advanced/can/common"
	"github.com/hovsep/fmesh-examples/simulation/can_bus/advanced/can/controller"
	"github.com/hovsep/fmesh-examples/simulation/can_bus/advanced/microcontroller"
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
func NewNode(unitName string, mcuInitState func(state component.State), mcuActivationFunction component.ActivationFunc) (*Node, error) {
	mcu, err := microcontroller.New(unitName, mcuInitState, mcuActivationFunction)
	if err != nil {
		return nil, fmt.Errorf("node %s: mcu: %w", unitName, err)
	}
	ctl, err := controller.New(unitName)
	if err != nil {
		return nil, fmt.Errorf("node %s: controller: %w", unitName, err)
	}
	trsv, err := NewTransceiver(unitName)
	if err != nil {
		return nil, fmt.Errorf("node %s: transceiver: %w", unitName, err)
	}

	if err := mcu.OutputByName(common.PortCANTx).PipeTo(ctl.InputByName(common.PortCANTx)); err != nil {
		return nil, fmt.Errorf("node %s: mcu→ctl: %w", unitName, err)
	}
	if err := ctl.OutputByName(common.PortCANRx).PipeTo(mcu.InputByName(common.PortCANRx)); err != nil {
		return nil, fmt.Errorf("node %s: ctl→mcu: %w", unitName, err)
	}

	if err := ctl.OutputByName(common.PortCANTx).PipeTo(trsv.InputByName(common.PortCANTx)); err != nil {
		return nil, fmt.Errorf("node %s: ctl→trsv: %w", unitName, err)
	}
	if err := trsv.OutputByName(common.PortCANRx).PipeTo(ctl.InputByName(common.PortCANRx)); err != nil {
		return nil, fmt.Errorf("node %s: trsv→ctl: %w", unitName, err)
	}

	return &Node{
		MCU:         mcu,
		Controller:  ctl,
		Transceiver: trsv,
	}, nil
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
func (nodes Nodes) ConnectToBus(b *bus.Bus) error {
	for _, node := range nodes {
		if err := node.Transceiver.OutputByName(common.PortCANL).PipeTo(b.Wires.InputByName(common.PortCANL)); err != nil {
			return fmt.Errorf("transceiver→bus CAN_L: %w", err)
		}
		if err := node.Transceiver.OutputByName(common.PortCANH).PipeTo(b.Wires.InputByName(common.PortCANH)); err != nil {
			return fmt.Errorf("transceiver→bus CAN_H: %w", err)
		}

		if err := b.Wires.OutputByName(common.PortCANL).PipeTo(node.Transceiver.InputByName(common.PortCANL)); err != nil {
			return fmt.Errorf("bus→transceiver CAN_L: %w", err)
		}
		if err := b.Wires.OutputByName(common.PortCANH).PipeTo(node.Transceiver.InputByName(common.PortCANH)); err != nil {
			return fmt.Errorf("bus→transceiver CAN_H: %w", err)
		}

		if err := node.Controller.OutputByName(common.PortControllerState).PipeTo(b.Watchdog.InputByName(common.PortControllerState)); err != nil {
			return fmt.Errorf("ctl→watchdog: %w", err)
		}

	}
	return nil
}
