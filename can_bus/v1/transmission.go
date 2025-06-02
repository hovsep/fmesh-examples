package main

import "github.com/hovsep/fmesh/component"

const (
	ecuTCM = "tcm"
)

func getTCM(bus *component.Component) *CanNode {
	return getCanNode(ecuTCM, bus, func(state component.State) {
		state.Set(ecuMemCanID, 0x120)
		state.Set(ecuMemLog, []string{})
	},
		func(this *component.Component) error {
			return nil
		})
}
