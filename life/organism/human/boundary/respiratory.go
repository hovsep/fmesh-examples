package boundary

import (
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

func GetRespiratory() *component.Component {
	return component.New("boundary:respiratory").
		WithDescription("Transforms environmental gas signals into chemical levels and lung input for circulation").
		AddInputs(
			"time",
			"environmental_gas",
		).
		AddOutputs(
			"inspired_gas", // to lungs
		). // TODO: use PipelineActivationFunc to transform gas (filter, humidify, warm up)
		WithActivationFunc(helper.PipelineActivationFunc([]string{"environmental_gas"}, "inspired_gas", sp1, dummySignalProcessor, sp2, sp3))
}

func dummySignalProcessor(sigs *signal.Group) (*signal.Group, error) {
	return sigs, nil
}

// This one adds labels
func sp1(sigs *signal.Group) (*signal.Group, error) {
	return sigs.Map(func(signal *signal.Signal) *signal.Signal {
		return signal.AddLabel("stage", "sp1").AddLabel("todo", "will be removed in sp2")
	}), nil
}

// Adds signals and removes labels
func sp2(sigs *signal.Group) (*signal.Group, error) {
	return sigs.Map(func(signal *signal.Signal) *signal.Signal {
		return signal.WithoutLabels("todo")
	}).AddFromPayloads(111, 222, 333), nil
}

// Multiplies ints
func sp3(sigs *signal.Group) (*signal.Group, error) {
	return sigs.MapPayloads(func(payload any) any {
		_, isInt := payload.(int)
		if isInt {
			return payload.(int) * 2
		}
		return payload
	}), nil
}
