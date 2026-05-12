package boundary

import (
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

//@TODO: reducing any level of air signal brakes the sum()=100% rule as we do not have any rebalance\renormalization in place
// this must be fixed in generalized way, we need some tooling to model mixtures with explicit Filter("axis", x%) api which will rebalance compounds
// ideally it must support both gas or fluid mixtures, but can be 2 separate packages as well

func GetRespiratory() *component.Component {
	return component.New("boundary:respiratory").
		WithDescription("Transforms environmental gas signals into chemical levels and lung input for circulation").
		AddInputs(
			"time",
			"environmental_gas",
		).
		AddOutputs(
			"inspired_gas",
		).
		WithActivationFunc(
			helper.PipelineActivationFunc([]string{"environmental_gas"}, "inspired_gas", filterInspiredGas, humidifyInspiredGas, warmUpInspiredGas))
}

// Applies pollution reduction.
func filterInspiredGas(sigs *signal.Group) (*signal.Group, error) {
	return getRespiratoryEffect("pollution", func(p float64) float64 {
		return p * 0.5
	})(sigs)
}

// Applies humidity increase.
func humidifyInspiredGas(sigs *signal.Group) (*signal.Group, error) {
	return getRespiratoryEffect("humidity", func(h float64) float64 {
		return h * 1.1
	})(sigs)
}

// Applies temperature increase.
func warmUpInspiredGas(sigs *signal.Group) (*signal.Group, error) {
	return getRespiratoryEffect("temperature", func(t float64) float64 {
		return t + 0.2
	})(sigs)
}

// getRespiratoryEffect returns a pipeline stage function that applies a basic transformation to air signal.
func getRespiratoryEffect(axis string, mapFunc func(old float64) float64) helper.PipeLineStageFunction {
	return func(respiratorySignals *signal.Group) (*signal.Group, error) {
		return respiratorySignals.Map(func(s *signal.Signal) *signal.Signal {
			if !helper.IsAir(s) {
				// No change
				return s
			}

			return helper.MapAirLevel(s, axis, mapFunc)
		}), nil
	}
}
