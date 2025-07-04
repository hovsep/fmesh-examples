package controller

import "fmt"

// State is the main DFSM of CAN-controller
type State byte

const (
	stateIdle State = iota
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

func (state State) String() string {
	return controllerStateNames[state]
}

func (state State) To(next State) string {
	return fmt.Sprintf("%s->%s", state, next)
}
