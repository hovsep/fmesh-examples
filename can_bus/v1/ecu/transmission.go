package ecu

import (
	"github.com/hovsep/fmesh-examples/can_bus/v1/can"
	"github.com/hovsep/fmesh/component"
)

const (
	TCMUnitName = "tcm"
	TCMNodeID   = 0x120
)

func NewTCM() *can.Node {
	return can.NewNode(TCMUnitName, func(state component.State) {
		state.Set(ecuMemCanID, TCMNodeID)
		state.Set(ecuMemLog, []string{})
	},
		func(this *component.Component) error {
			return nil
		})
}
