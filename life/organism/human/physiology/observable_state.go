package physiology

import (
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/component"
)

type BreathingPhase string

const (
	LastBrainActivity                   common.State = "last_brain_activity"
	defaultBrainActivitySmoothingFactor              = 0.1    // alpha in ema
	defaultBrainActivityThreshold                    = 0.0001 // epsilon in ema

	Inhale BreathingPhase = "inhale"
	Exhale BreathingPhase = "exhale"
)

// GetObservableState ...
func GetObservableState() *component.Component {
	return component.New("physiology:observable_state").
		WithDescription("Observable state of the human being (e.g., temperature, blood pressure etc)").
		WithInitialState(func(st component.State) {
			st.Set(LastBrainActivity, 0.0)
		}).
		AddInputs(
			"time",
			"brain_activity",
			"heart_cardiac_activation",
			"heart_rate",

			"lung_left_exhaled_gas",
			"lung_left_phase",
			"lung_left_volume",
			"lung_left_alveolar_pressure",
			"lung_left_pleural_pressure",
			"lung_left_gas_composition",
			"lung_left_respiratory_rate",
			"lung_left_inspiration_duration",
			"lung_left_exhalation_duration",
			"lung_left_lung_efficiency",
			"lung_left_alveolar_dead_space",
			"lung_left_stretch_ratio",

			"lung_right_exhaled_gas",
			"lung_right_phase",
			"lung_right_volume",
			"lung_right_alveolar_pressure",
			"lung_right_pleural_pressure",
			"lung_right_gas_composition",
			"lung_right_respiratory_rate",
			"lung_right_inspiration_duration",
			"lung_right_exhalation_duration",
			"lung_right_lung_efficiency",
			"lung_right_alveolar_dead_space",
			"lung_right_stretch_ratio",
		).
		AddOutputs(
			"is_alive",
			"brain_activity",
			"brain_activity_trend",
			"heart_cardiac_activation",
			"heart_rate",
			"breathing_phase").
		WithActivationFunc(composeActivationFunction(
			handleBrainSignals,
			handleHeartSignals,
			handleLungsSignals,
		))
}

func handleBrainSignals(this *component.Component) error {
	var isAlive bool

	defer func() {
		this.OutputByName("is_alive").PutPayloads(isAlive)
	}()

	if !this.InputByName("brain_activity").HasSignals() {
		// If we don't have brain activity signals, we are dead
		return nil
	}

	isAlive = true

	// Calculate brain activity trend
	currentBrainActivity := helper.AsF64(this.InputByName("brain_activity").Signals().First())
	lastSmoothedBrainActivity := this.State().Get(LastBrainActivity).(float64)

	// Exponential Moving Average helps to determine trend without storing historical data
	ema := helper.NewEMA(defaultBrainActivitySmoothingFactor, lastSmoothedBrainActivity, defaultBrainActivityThreshold)
	smoothedBrainActivity := ema.Update(currentBrainActivity)
	brainActivityTrend := ema.ClassifyTrend(currentBrainActivity)

	this.State().Set(LastBrainActivity, smoothedBrainActivity)
	this.OutputByName("brain_activity").PutPayloads(smoothedBrainActivity)
	this.OutputByName("brain_activity_trend").PutPayloads(brainActivityTrend)
	return nil
}

func handleHeartSignals(this *component.Component) error {
	return helper.MultiForward(
		helper.PortPair{
			this.InputByName("heart_cardiac_activation"),
			this.OutputByName("heart_cardiac_activation"),
		},
		helper.PortPair{
			this.InputByName("heart_rate"),
			this.OutputByName("heart_rate"),
		})
}

func handleLungsSignals(this *component.Component) error {
	var phase BreathingPhase

	phase = Inhale

	this.OutputByName("breathing_phase").PutPayloads(phase)
	return nil
}

// composeActivationFunction allows composing multiple activation functions into one
func composeActivationFunction(funcs ...component.ActivationFunc) component.ActivationFunc {
	return func(this *component.Component) error {
		for _, f := range funcs {
			if err := f(this); err != nil {
				return err
			}
		}
		return nil
	}
}
