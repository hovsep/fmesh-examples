package main

import "github.com/hovsep/fmesh/component"

const (
	ecuACU = "acu"
)

func getACU(bus *component.Component) *CanNode {
	return getCanNode(ecuACU, bus, func(state component.State) {
		state.Set(ecuMemCanID, 0x1A0)
	},
		func(this *component.Component) error {
			return nil
		})
}
