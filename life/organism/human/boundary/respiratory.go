package boundary

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// @TODO: maybe we should imbibe some toxins from inspired air in this component (smoking, smelling toxins or pollen\allergens)
func GetRespiratory() (*component.Component, error) {
	c, err := component.New("boundary:respiratory",
		component.WithDescription("Represents path from nose to trachea.Transforms environmental gas signals into chemical levels and lung input for circulation"),
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
	result := sigs.MapIf(func(s *signal.Signal) bool {
		return s.Labels().ValueIs("category", "gas") && s.Labels().ValueIs("type", "air")
	}, func(airSignal *signal.Signal) *signal.Signal {
		return helper.MapAirScalar(airSignal, "composition:pollution", func(p float64) float64 {
			return p * 0.5
		})
	})
	return result, nil
}

// Applies humidity increase.
func humidifyInspiredGas(sigs *signal.Group) (*signal.Group, error) {
	result := sigs.MapIf(func(s *signal.Signal) bool {
		return s.Labels().ValueIs("category", "gas") && s.Labels().ValueIs("type", "air")
	}, func(airSignal *signal.Signal) *signal.Signal {
		return helper.MapAirScalar(airSignal, "humidity", func(h float64) float64 {
			return h * 1.1
		})
	})
	return result, nil
}

// Applies temperature increase.
func warmUpInspiredGas(sigs *signal.Group) (*signal.Group, error) {
	result := sigs.MapIf(func(s *signal.Signal) bool {
		return s.Labels().ValueIs("category", "gas") && s.Labels().ValueIs("type", "air")
	}, func(airSignal *signal.Signal) *signal.Signal {
		return helper.MapAirScalar(airSignal, "temperature", func(t float64) float64 {
			return t + 0.2
		})
	})
	return result, nil
}
