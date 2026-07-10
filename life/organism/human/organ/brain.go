package organ

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	da "github.com/hovsep/fmesh-examples/life/organism/human/distributed_anatomy"
	"github.com/hovsep/fmesh-examples/life/plugin/damage"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/component"
)

const (
	NeuralDrive       common.State = "neural_drive"
	NeuralDriveJitter              = 0.02 * DNCS
	MinNeuralDrive                 = 0.0 * DNCS
	MaxNeuralDrive                 = 1.0 * DNCS

	// 0.0 - 0.2 Sleep
	// 0.3 - 0.6 Baseline activity
	// 0.7 - 1.0 Stress, exercise, threat
	defaultNeuralDrive = 0.3 * DNCS

	// Metabolism: the brain is O2-hungry and returns CO2 to the blood (rates in %/s).
	BrainO2Consumption = 3.0 * PercentPerSecond
	BrainCO2Production = 3.0 * PercentPerSecond
)

func GetBrain() (*component.Component, error) {
	c, err := component.New("organ:brain",
		component.WithDescription("The Brain"),
		component.WithPlugins(damage.New()),
		component.WithInputs("time", "blood"),
		component.WithOutputs("neural_drive", "blood"),
		component.WithActivationFunc(helper.SequentialActivationFunc(
			oscillateNeuralDrive,
			emitBrainMetabolism,
		)),
		component.WithInitialState(func(state component.State) {
			state.Set(NeuralDrive, defaultNeuralDrive)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("organ:brain: %w", err)
	}
	return c, nil
}

func oscillateNeuralDrive(this *component.Component) error {
	// Only advance on a time tick; blood-only activations (from the shared bus) are ignored.
	if !this.InputByName("time").HasSignals() {
		return nil
	}

	var nextND float64

	this.State().Update(NeuralDrive, func(currentND any) any {
		nextND = helper.Clamp(helper.Jitter(currentND.(float64), NeuralDriveJitter), MinNeuralDrive, MaxNeuralDrive)
		return nextND
	})

	return this.OutputByName("neural_drive").PutPayloads(nextND)
}

// emitBrainMetabolism secretes the brain's O2 demand and CO2 output into the blood bus.
// Gated on time so it fires exactly once per tick (not on other input activations).
func emitBrainMetabolism(this *component.Component) error {
	if !this.InputByName("time").HasSignals() {
		return nil
	}
	return this.OutputByName("blood").PutSignals(
		da.Secretion(da.SubstanceO2Consumption, BrainO2Consumption),
		da.Secretion(da.SubstanceCO2Production, BrainCO2Production),
	)
}
