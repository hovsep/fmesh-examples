package physiology

import (
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh/signal"
)

// isLevel checks if a signal represents a level
func isLevel(s *signal.Signal) bool {
	return s.Labels().ValueIs(common.Type, common.Level)
}

// NewLevel builds a signal that represents a level
func NewLevel(value float64, axis string) *signal.Signal {
	return signal.New(value).AddLabel(common.Type, common.Level).AddLabel(common.Axis, axis)
}
