package organ

import (
	"math"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/component"
)

const (
	cardiacActivation  common.State = "cardiac_activation"
	rate               common.State = "rate"
	phaseState         common.State = "phase"
	defaultCardiacBias float64      = 0.5 // Used when autonomic tone is not received (denervation, death, etc)
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
			state.Set(cardiacActivation, 0.0) // Initial cardiac activation
			state.Set(rate, 60)               // Default BPM
			state.Set(phaseState, 0.0)        // Phase in current heartbeat cycle
		}).
		AddInputs("time", "autonomic_tone").
		AddOutputs("cardiac_activation", "rate").
		WithActivationFunc(func(this *component.Component) error {
			if !this.InputByName("time").HasSignals() {
				// Nothing happens
				return nil
			}

			if !this.InputByName("autonomic_tone").HasSignals() {
				// TODO: handle denervation, death, etc
			}

			dt, err := helper.TickDurationInSec(this.InputByName("time").Signals().First())
			if err != nil {
				return err
			}

			cardiacBias := defaultCardiacBias

			if this.InputByName("autonomic_tone").HasSignals() {
				cardiacBias, err = helper.GetBias(this.InputByName("autonomic_tone").Signals().First(), common.Cardiac)
				if err != nil {
					return err
				}
			}

			// Map cardiac bias → BPM
			minBPM := 50.0
			maxBPM := 120.0
			bpm := minBPM + (maxBPM-minBPM)*cardiacBias

			// Update rate
			this.State().Update(rate, func(v any) any { return int(bpm) })

			// Advance phase
			currentPhase := this.State().Get(phaseState).(float64)
			phaseStep := dt / (60.0 / bpm)
			newPhase := math.Mod(currentPhase+phaseStep, 1.0)
			this.State().Set(phaseState, newPhase)

			// Compute cardiac activation
			act := CardiacActivationWave(newPhase)
			this.State().Set(cardiacActivation, act)

			// Output signals
			this.OutputByName("cardiac_activation").PutPayloads(act)
			this.OutputByName("rate").PutPayloads(int(bpm))

			return nil
		})
}
