package controller

import "fmt"

// State is the main DFSM of CAN-controller
type State byte

// StateMap holds a state of controllers by its names
type StateMap map[string]State

const (
	StateIdle State = iota
	StateWaitForBusIdle
	StateArbitration
	StateTransmit
	StateReceive
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

func (stateMap StateMap) MergeFrom(sourceStateMap StateMap) StateMap {
	for name, state := range sourceStateMap {
		stateMap[name] = state
	}

	return stateMap
}
