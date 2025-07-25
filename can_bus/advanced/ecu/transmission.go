package ecu

import (
	"github.com/hovsep/fmesh-examples/can_bus/advanced/internal/can"
)

const (
	TCMUnitName = "tcm"
)

func NewTCM() *can.Node {
	return can.NewNode(TCMUnitName, nil, nil)
}
