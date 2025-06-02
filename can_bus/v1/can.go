package main

import (
	"errors"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
)

const (
	portCANTx = "can_tx" // Transmit to CAN bus
	portCANRx = "can_rx" // Receive from CAN bus
	portCANH  = "can_h"  // CAN high
	portCANL  = "can_l"  // CAN low
)

// CanNode consists of multiple components
type CanNode struct {
	MCU         *component.Component // Main logic, operates with frames
	Controller  *component.Component // Converts frames to bits and vice versa
	Transceiver *component.Component // Converts bits to voltages
}

type CanNodesSlice []*CanNode

func getBus() *component.Component {
	return component.New("can_bus").
		WithInputs(portCANL, portCANH).
		WithOutputs(portCANL, portCANH).
		WithActivationFunc(func(this *component.Component) error {
			errL := port.ForwardSignals(this.InputByName(portCANL), this.OutputByName(portCANL))
			errH := port.ForwardSignals(this.InputByName(portCANH), this.OutputByName(portCANH))

			if errL != nil {
				return errL
			}

			if errH != nil {
				return errH
			}

			return nil
		})
}

// Creates a new CAN node
func getCanNode(unitName string, bus *component.Component, initState func(state component.State), mcuLogic component.ActivationFunc) *CanNode {
	// Create electronic components
	mcu := getMCU(unitName, initState, mcuLogic)
	controller := getController(unitName)
	transceiver := getTransceiver(unitName)

	// Wiring : mcu <--> controller <--> transceiver <--> bus

	// mcu -> controller:
	mcu.OutputByName(portCANTx).PipeTo(controller.InputByName(portCANTx))
	// mcu <- controller
	controller.OutputByName(portCANRx).PipeTo(mcu.InputByName(portCANRx))

	// controller -> transceiver
	controller.OutputByName(portCANTx).PipeTo(transceiver.InputByName(portCANTx))
	// controller <- transceiver
	transceiver.OutputByName(portCANRx).PipeTo(controller.InputByName(portCANRx))

	// transceiver -> bus:
	transceiver.OutputByName(portCANL).PipeTo(bus.InputByName(portCANL))
	transceiver.OutputByName(portCANH).PipeTo(bus.InputByName(portCANH))
	// transceiver <- bus:
	bus.OutputByName(portCANL).PipeTo(transceiver.InputByName(portCANL))
	bus.OutputByName(portCANH).PipeTo(transceiver.InputByName(portCANH))

	return &CanNode{
		MCU:         mcu,
		Controller:  controller,
		Transceiver: transceiver,
	}
}

// MCU - microcontroller unit
// (executes embedded logic, exchanges digital signals (bits) with plugged CAN transceiver)
func getMCU(name string, initState func(state component.State), logic component.ActivationFunc) *component.Component {
	return component.New(getMCUComponentName(name)).
		WithInitialState(initState).
		WithActivationFunc(logic).
		WithInputs(portCANRx). // Frame in
		WithOutputs(portCANTx) // Frame out

}

// Converts frames to bits and vice versa
func getController(unitName string) *component.Component {
	return component.New(getControllerComponentName(unitName)).
		WithActivationFunc(func(this *component.Component) error {
			// Write path (mcu writes to bus)
			for _, sig := range this.InputByName(portCANTx).AllSignalsOrNil() {
				frame, ok := sig.PayloadOrNil().(*CanFrame)
				if !ok || !frame.isValid() {
					return errors.New("controller received corrupted frame")
				}

				bits := frame.toBits().String()
				_ = bits
			}
			return nil
		}).
		WithInputs(portCANTx, portCANRx). // Frame in, bits in
		WithOutputs(portCANTx, portCANRx) // Bits out, frame out
}

// Converts bits to voltage and vice versa
func getTransceiver(unitName string) *component.Component {
	return component.New(getTransceiverComponentName(unitName)).
		WithActivationFunc(func(this *component.Component) error {
			return nil
		}).
		WithInputs(portCANTx, portCANH, portCANL). // Bits in (write to bus), voltage in (read from bus)
		WithOutputs(portCANRx, portCANH, portCANL) // Bits out (read from bus), voltage out (write to bus)
}

// Converts CanNodesSlice to slice of components
func (canNodes CanNodesSlice) getAllComponents() []*component.Component {
	var all []*component.Component
	for _, ecu := range canNodes {
		all = append(all, ecu.MCU, ecu.Controller, ecu.Transceiver)
	}
	return all
}
