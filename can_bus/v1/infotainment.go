package main

import "github.com/hovsep/fmesh/component"

const (
	ecuHU = "infotainment-hu"
)

func getHU(bus *component.Component) *CanNode {
	return getCanNode(ecuHU, bus, func(state component.State) {
		state.Set(ecuMemCanID, 0x300)
	},
		func(this *component.Component) error {
			return nil
		})
}
