package ecu

import (
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can"
	"github.com/hovsep/fmesh/component"
)

const (
	ACUUnitName = "acu"
	ACUNodeID   = 0x1A0
)

func NewACU() *can.Node {
	return can.NewNode(ACUUnitName, func(state component.State) {
		state.Set(ecuMemCanID, ACUNodeID)
	},
		func(this *component.Component) error {
			return nil
		})
}
