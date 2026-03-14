package organ

import (
	"fmt"
	"math"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/component"
)

const (
	cardiacBias        common.State = "cardiac_bias"
	rate               common.State = "rate"
	phase              common.State = "phase"
	defaultCardiacBias float64      = 0.5 // Used when autonomic tone is not received (denervation, death, etc)
	minBPM             float64      = 40
	maxBPM             float64      = 200
)

// CardiacActivationWave returns normalized contraction amplitude for a given phase
func CardiacActivationWave(phase float64) float64 {
	if phase < 0.1 {
		return math.Exp(-30 * phase) // sharp spike
	}
	return 0
}

// GetHeart returns heart component
func GetHeart() *component.Component {
	return component.New("organ:heart").
		WithDescription("Heart").
		WithInitialState(func(state component.State) {
			state.Set(cardiacBias, defaultCardiacBias)
			state.Set(rate, 60)   // Initial BPM
			state.Set(phase, 0.0) // Phase in the current heartbeat cycle
		}).
		AddInputs("time", "autonomic_tone").
		AddOutputs("cardiac_activation", "rate").
		WithActivationFunc(func(this *component.Component) error {
			if this.InputByName("time").HasSignals() {
				oscErr := oscillate(this)
				if oscErr != nil {
					return oscErr
				}
			}

			if this.InputByName("autonomic_tone").HasSignals() {
				ATErr := handleAutonomicTone(this)
				if ATErr != nil {
					return ATErr
				}
			}

			// Output signals
			this.OutputByName("rate").PutPayloads(this.State().Get(rate).(int))
			return nil
		})
}

func oscillate(this *component.Component) error {
	if !this.InputByName("time").HasSignals() {
		return fmt.Errorf("can not oscillate heart: no time signal")
	}

	dt, err := helper.TickDurationInSec(this.InputByName("time").Signals().First())
	if err != nil {
		return err
	}

	bias := this.State().Get(cardiacBias).(float64)

	// Update rate
	var bpm float64
	this.State().Update(rate, func(v any) any {
		bpm = minBPM + (maxBPM-minBPM)*bias
		return int(bpm)
	})

	// Advance phase
	var nextPhase float64
	this.State().Update(phase, func(old any) any {
		currentPhase := this.State().Get(phase).(float64)
		phaseStep := dt / (60.0 / bpm)
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
		this.State().Set(cardiacBias, defaultCardiacBias)
	}

	this.State().Update(cardiacBias, func(v any) any {
		if !this.InputByName("autonomic_tone").HasSignals() {
			return defaultCardiacBias
		}

		bias, err := helper.GetBias(this.InputByName("autonomic_tone").Signals().First(), common.Cardiac)
		if err != nil {
			this.Logger().Println("Failed to get cardiac bias from autonomic tone: ", err)
			return defaultCardiacBias
		}

		return bias
	})
	return nil
}
