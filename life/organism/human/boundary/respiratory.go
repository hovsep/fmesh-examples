package boundary

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// @TODO: emit inspired gas only in inhale phase (derive from diaphragm)
func GetRespiratory() (*component.Component, error) {
	c, err := component.New("boundary:respiratory",
		component.WithDescription("Transforms environmental gas signals into chemical levels and lung input for circulation"),
		component.WithInputs(
			"time",
			"environmental_gas",
		),
		component.WithOutputs(
			"inspired_gas",
		),
		component.WithActivationFunc(
			helper.PipelineActivationFunc([]string{"environmental_gas"}, "inspired_gas", filterInspiredGas, humidifyInspiredGas, warmUpInspiredGas)),
	)
	if err != nil {
		return nil, fmt.Errorf("boundary:respiratory: %w", err)
	}
	return c, nil
}

// Applies pollution reduction.
func filterInspiredGas(sigs *signal.Group) (*signal.Group, error) {
	var mapErr error
	result := sigs.MapIf(helper.IsAir, func(airSignal *signal.Signal) *signal.Signal {
		modified, err := helper.MapAirComposition(airSignal, "pollution", func(p float64) float64 {
			return p * 0.5
		})
		if err != nil {
			mapErr = err
			return airSignal
		}
		return modified
	})
	if mapErr != nil {
		return nil, mapErr
	}
	return result, nil
}

// Applies humidity increase.
func humidifyInspiredGas(sigs *signal.Group) (*signal.Group, error) {
	var mapErr error
	result := sigs.MapIf(helper.IsAir, func(airSignal *signal.Signal) *signal.Signal {
		modified, err := helper.MapAirLevel(airSignal, "humidity", func(h float64) float64 {
			return h * 1.1
		})
		if err != nil {
			mapErr = err
			return airSignal
		}
		return modified
	})
	if mapErr != nil {
		return nil, mapErr
	}
	return result, nil
}

// Applies temperature increase.
func warmUpInspiredGas(sigs *signal.Group) (*signal.Group, error) {
	var mapErr error
	result := sigs.MapIf(helper.IsAir, func(airSignal *signal.Signal) *signal.Signal {
		modified, err := helper.MapAirLevel(airSignal, "temperature", func(t float64) float64 {
			return t + 0.2
		})
		if err != nil {
			mapErr = err
			return airSignal
		}
		return modified
	})
	if mapErr != nil {
		return nil, mapErr
	}
	return result, nil
}
