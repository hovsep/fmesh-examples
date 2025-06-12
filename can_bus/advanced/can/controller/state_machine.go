package controller

import "github.com/hovsep/fmesh/component"

// ControllerState is the main DFSM of can controller
type ControllerState byte

// ControllerTxState is a sub-DFSM determining which portion of data is being transmitted
type ControllerTxState byte

// ControllerRxState is a sub-DFSM determining which portion of data is being received
type ControllerRxState byte

const (
	controllerStateIdle ControllerState = iota
	controllerStateWaitForBusIdle
	controllerStateArbitration
	controllerStateTransmit
	controllerStateReceive

	controllerTxState ControllerTxState = iota
	controllerTxStateSOF
	controllerTxStateID
	controllerTxStateDLC
	controllerTxStateData
	controllerTxStateEOF
	controllerTxStateIFS

	controllerRxState ControllerRxState = iota
	controllerRxStateSOF
	controllerRxStateID
	controllerRxStateDLC
	controllerRxStateData
	controllerRxStateEOF
	controllerRxStateIFS
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
