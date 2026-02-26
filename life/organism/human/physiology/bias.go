package physiology

import (
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh/signal"
)

// isBias checks if a signal represents a regional bias
func isBias(s *signal.Signal) bool {
	return s.Labels().ValueIs(common.Type, common.Bias)
}

// NewBias builds a signal that represents a regional bias
func NewBias(value float64, region string) *signal.Signal {
	return signal.New(value).AddLabel(common.Type, common.Bias).AddLabel(common.Region, region)
}
