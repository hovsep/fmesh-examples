package organ

import (
	"math"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/component"
)

const (
	rate   common.State = "rate" // Target rate in BPM (can be positive even after denervation)
	phase  common.State = "phase"
	minBPM float64      = 40
	maxBPM float64      = 200
)

// CardiacActivationWave returns ECG-style contraction amplitude for a given phase
func CardiacActivationWave(phase float64) float64 {
	if phase < 0.05 {
		return math.Exp(-30 * phase) // R-Peak (Spike)
	}
	if phase >= 0.05 && phase < 0.1 {
		return -0.2 * math.Sin((phase-0.05)*20) // Simple S-Wave dip
	}
	return 0
}

// GetHeart returns heart component
func GetHeart() *component.Component {
	return component.New("organ:heart").
		WithDescription("Heart").
		WithInitialState(func(state component.State) {
			state.Set(rate, 60)   // Initial BPM
			state.Set(phase, 0.0) // Phase in the current heartbeat cycle
		}).
		AddInputs("time", "autonomic_tone").
		AddOutputs("cardiac_activation", "rate").
		WithActivationFunc(func(this *component.Component) error {
			oscErr := oscillate(this)
			if oscErr != nil {
				return oscErr
			}

			ATErr := handleAutonomicTone(this)
			if ATErr != nil {
				return ATErr
			}

			// Always output heart rate
			this.OutputByName("rate").PutPayloads(this.State().Get(rate).(int))
			return nil
		})
}

func oscillate(this *component.Component) error {
	if !this.InputByName("time").HasSignals() {
		return nil
	}

	dt, err := helper.TickDurationInSec(this.InputByName("time").Signals().First())
	if err != nil {
		return err
	}

	// Advance phase
	var nextPhase float64
	this.State().Update(phase, func(old any) any {
		currentPhase := old.(float64)
		currentRate := this.State().Get(rate).(int)
		phaseStep := dt / (60.0 / float64(currentRate))
		nextPhase = math.Mod(currentPhase+phaseStep, 1.0)
		return nextPhase
	})

	// Compute cardiac activation
	act := CardiacActivationWave(nextPhase)
	this.OutputByName("cardiac_activation").PutPayloads(act)
	return nil
}

func handleAutonomicTone(this *component.Component) error {
	if !this.InputByName("autonomic_tone").HasSignals() {
		return nil
	}

	bias, err := helper.GetBias(this.InputByName("autonomic_tone").Signals().First(), common.Cardiac)
	if err != nil {
		return err
	}

	// Update rate
	this.State().Update(rate, func(v any) any {
		return int(minBPM + (maxBPM-minBPM)*bias)
	})

	return nil
}
