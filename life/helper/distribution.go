package helper

import (
	"fmt"
	"maps"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh/signal"
)

// DistributionMap is a map of levels to their distribution (all levels always sum up to 100%)
type DistributionMap map[string]float64

// NewDistribution builds a signal that represents a distribution of levels
func NewDistribution(distributionMap DistributionMap) (*signal.Signal, error) {
	sum := 0.0
	for v := range maps.Values(distributionMap) {
		sum += v
	}

	if sum != 100 {
		return nil, fmt.Errorf("distribution does not sum up to 100")
	}

	distGroup := signal.NewGroup()

	for axis, value := range distributionMap {
		distGroup = distGroup.With(NewLevel(value, axis))
	}

	return signal.New(distGroup).WithLabel(common.Type, "distribution"), nil
}

// RebalanceDistribution rebalances a distribution ensuring that all levels sum up to 100%.
// Each level is scaled proportionally, so the total remains 100%.
func RebalanceDistribution(s *signal.Signal) (*signal.Signal, error) {
	group, err := AsGroup(s)
	if err != nil {
		return nil, fmt.Errorf("rebalance distribution: %w", err)
	}

	sum := 0.0
	group.ForEach(func(level *signal.Signal) error {
		v, err := AsF64(level)
		if err != nil {
			return err
		}
		sum += v
		return nil
	})

	if sum == 0 {
		return nil, fmt.Errorf("cannot rebalance a distribution where all levels are zero")
	}

	scaleFactor := 100.0 / sum

	rebalanced := DistributionMap{}
	err = group.ForEach(func(level *signal.Signal) error {
		axis, _ := level.Labels().Value(common.Axis)
		v, err := AsF64(level)
		if err != nil {
			return err
		}
		rebalanced[axis] = v * scaleFactor
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("rebalance distribution: %w", err)
	}

	allLabels := s.Labels().All()
	result, err := NewDistribution(rebalanced)
	if err != nil {
		return nil, err
	}
	return result.WithLabels(maps.Clone(allLabels)), nil
}
