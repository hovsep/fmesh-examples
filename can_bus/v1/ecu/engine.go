package ecu

import (
	"github.com/hovsep/fmesh-examples/can_bus/v1/can"
	"github.com/hovsep/fmesh/component"
)

const (
	ECMUnitName                = "ecm"
	ECMNodeID                  = 0x100
	ecmMemVin                  = "vin"
	ecmMemOxygenSensorAdaptive = "oxsa"
)

func NewECM(bus *component.Component) *can.Node {
	return can.NewNode(ECMUnitName, bus, func(state component.State) {
		state.Set(ecuMemCanID, ECMNodeID)
		state.Set(ecuMemSerial, "1995-00-BB-LE")
		state.Set(ecuMemLog, []string{})
		state.Set(ecmMemVin, "JHMSL65848Z411439")
		state.Set(ecmMemOxygenSensorAdaptive, 0)
	},
		func(this *component.Component) error {
			return nil
		})
}
