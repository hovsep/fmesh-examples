package controller

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
