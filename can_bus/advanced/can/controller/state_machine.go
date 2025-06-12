package controller

import "github.com/hovsep/fmesh/component"

// ControllerState is the main DFSM of can controller
type ControllerState byte

const (
	stateIdle ControllerState = iota
	stateWaitForBusIdle
	stateArbitration
	stateTransmit
	stateReceive
)

var (
	controllerStateNames = []string{
		"IDLE",
		"WAITING FOR BUS IDLE",
		"ARBITRATION",
		"TRANSMIT",
		"RECEIVE",
	}
)

func (state ControllerState) String() string {
	return controllerStateNames[state]
}

func logCurrentState(controller *component.Component, currentState ControllerState) {
	controller.Logger().Println("current state:", currentState)
}

func logStateTransition(controller *component.Component, currentState, newState ControllerState) {
	controller.Logger().Printf("state transition: %s -> %s", currentState, newState)
}
