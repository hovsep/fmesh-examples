package helper

import (
	"maps"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh/signal"
)

// DistributionMap is a map of levels to their distribution (all levels always sum up to 100%)
type DistributionMap map[string]float64

// NewDistribution builds a signal that represents a distribution of levels
func NewDistribution(distributionMap DistributionMap) *signal.Signal {
	sum := 0.0
	for v := range maps.Values(distributionMap) {
		sum += v
	}

	if sum != 100 {
		panic("distribution does not sum up to 100")
	}

	distGroup := signal.NewGroup()

	for axis, value := range distributionMap {
		distGroup.Add(NewLevel(value, axis))
	}

	return signal.New(distGroup).AddLabel(common.Type, "distribution")
}

// RebalanceDistribution rebalances a distribution ensuring that all levels sum up to 100%.
// Each level is scaled proportionally, so the total remains 100%.
func RebalanceDistribution(s *signal.Signal) *signal.Signal {
	group := AsGroup(s)

	sum := 0.0
	group.ForEach(func(level *signal.Signal) error {
		sum += AsF64(level)
		return nil
	})

	if sum == 0 {
		panic("cannot rebalance a distribution where all levels are zero")
	}

	scaleFactor := 100.0 / sum

	rebalanced := DistributionMap{}
	group.ForEach(func(level *signal.Signal) error {
		axis, _ := level.Labels().Value(common.Axis)
		rebalanced[axis] = AsF64(level) * scaleFactor
		return nil
	})

	allLabels, _ := s.Labels().All()
	return NewDistribution(rebalanced).AddLabels(maps.Clone(allLabels))
}
