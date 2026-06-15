package organ

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
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
)

func GetBrain() (*component.Component, error) {
	c, err := component.New("organ:brain",
		component.WithDescription("The Brain"),
		component.WithInputs("time"),
		component.WithOutputs("neural_drive", "failure"),
		component.WithActivationFunc(
			helper.SequentialActivationFunc(
				handleAging,
				oscillateNeuralDrive,
			),
		),
		component.WithInitialState(func(state component.State) {
			state.Set(common.DamageLevel, defaultDamageLevel)
			state.Set(NeuralDrive, defaultNeuralDrive)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("organ:brain: %w", err)
	}
	return c, nil
}

func handleAging(this *component.Component) error {
	var currentDamage float64

	this.State().Update(common.DamageLevel, func(oldDamage any) any {
		currentDamage = oldDamage.(float64)
		return currentDamage + damageRampRate
	})

	if currentDamage >= criticalDamageLevel {
		return this.OutputByName("failure").PutSignals(signal.New("brain_failure").WithLabel("type", "acute"))
	}

	return nil
}

func oscillateNeuralDrive(this *component.Component) error {
	var nextND float64

	this.State().Update(NeuralDrive, func(currentND any) any {
		nextND = helper.Clamp(helper.Jitter(currentND.(float64), NeuralDriveJitter), MinNeuralDrive, MaxNeuralDrive)
		return nextND
	})

	return this.OutputByName("neural_drive").PutPayloads(nextND)
}
