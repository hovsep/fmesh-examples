package helper

import (
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh/signal"
)

// IsLevel checks if a signal represents a level
func IsLevel(s *signal.Signal) bool {
	return s.Labels().ValueIs(common.Type, common.Level)
}

// NewLevel builds a signal that represents a level
func NewLevel(value float64, axis string) *signal.Signal {
	return signal.New(value).WithLabel(common.Type, common.Level).WithLabel(common.Axis, axis)
}

// IsLevelWithAxis checks if a signal represents a level with a specific axis
func IsLevelWithAxis(s *signal.Signal, axis string) bool {
	return IsLevel(s) && s.Labels().ValueIs(common.Axis, axis)
}
