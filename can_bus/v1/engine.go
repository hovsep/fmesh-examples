package main

import "github.com/hovsep/fmesh/component"

const (
	ecuECM                     = "ecm"
	ecmMemVin                  = "vin"
	ecmMemOxygenSensorAdaptive = "oxsa"
)

func getECM(bus *component.Component) *CanNode {
	return getCanNode(ecuECM, bus, func(state component.State) {
		state.Set(ecuMemCanID, 0x100)
		state.Set(ecuMemSerial, "1995-00-BB-LE")
		state.Set(ecuMemLog, []string{})
		state.Set(ecmMemVin, "JHMSL65848Z411439")
		state.Set(ecmMemOxygenSensorAdaptive, 0)
	},
		func(this *component.Component) error {
			return nil
		})
}
