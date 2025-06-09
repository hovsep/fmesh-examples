package can

import "github.com/hovsep/fmesh/component"

type ControllerState byte

const (
	controllerStateIdle ControllerState = iota
	controllerStateWaitForBusIdle
	controllerStateArbitration
	controllerStateTransmit
	controllerStateReceive
)

var controllerStateNames = []string{
	"IDLE",
	"WAITING FOR BUS IDLE",
	"ARBITRATION",
	"TRANSMIT",
	"RECEIVE",
}

func (state ControllerState) String() string {
	return controllerStateNames[state]
}

func logCurrentState(controller *component.Component, currentState ControllerState) {
	controller.Logger().Println("current state:", currentState)
}

func logStateTransition(controller *component.Component, currentState, newState ControllerState) {
	controller.Logger().Printf("state transition: %s -> %s", currentState, newState)
}
