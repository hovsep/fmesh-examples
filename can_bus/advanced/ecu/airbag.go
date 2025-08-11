package ecu

import (
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can"
)

const (
	ACUUnitName = "acu"
)

func NewACU() *can.Node {
	return can.NewNode(ACUUnitName, nil, nil)
}
