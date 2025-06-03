package ecu

import (
	"github.com/hovsep/fmesh-examples/can_bus/v1/can"
	"github.com/hovsep/fmesh/component"
)

const (
	HUUnitName = "infotainment-hu"
	HUNodeID   = 0x300
)

func NewHU(bus *component.Component) *can.Node {
	return can.NewNode(HUUnitName, bus, func(state component.State) {
		state.Set(ecuMemCanID, HUNodeID)
	},
		func(this *component.Component) error {
			return nil
		})
}
