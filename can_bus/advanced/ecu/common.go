package ecu

import (
	"fmt"
	"github.com/hovsep/fmesh/component"
)

const (
	EcuMemLog = "log" // Internal log

	ObdFunctionalRequestID = 0x7DF // Functional (broadcast) request
)

func setParam(state component.State, pid uint8, value any) {
	state.Set(paramStateKey(pid), value)
}

func getParam(state component.State, pid uint8) any {
	return state.Get(paramStateKey(pid))
}

func paramStateKey(pid uint8) string {
	return fmt.Sprintf("param-%02X", pid)
}
