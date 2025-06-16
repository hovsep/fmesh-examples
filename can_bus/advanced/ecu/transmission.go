package ecu

import (
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can"
	"github.com/hovsep/fmesh/component"
)

const (
	TCMUnitName = "tcm"
)

func NewTCM() *can.Node {
	return can.NewNode(TCMUnitName, func(state component.State) {
		state.Set(EcuMemLog, []string{})
	},
		func(this *component.Component) error {
			return nil
		})
}
