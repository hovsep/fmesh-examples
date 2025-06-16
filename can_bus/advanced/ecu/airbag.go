package ecu

import (
	"github.com/hovsep/fmesh-examples/can_bus/advanced/can"
	"github.com/hovsep/fmesh/component"
)

const (
	ACUUnitName = "acu"
)

func NewACU() *can.Node {
	return can.NewNode(ACUUnitName, func(state component.State) {
	},
		func(this *component.Component) error {
			return nil
		})
}
