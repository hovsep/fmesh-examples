package physiology

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/component"
)

const (
	LastBrainActivity                   common.State = "last_brain_activity"
	defaultBrainActivitySmoothingFactor              = 0.1 * DNCS    // alpha in ema
	defaultBrainActivityThreshold                    = 0.0001 * DNCS // epsilon in ema
)

// GetObservableState ...
func GetObservableState() (*component.Component, error) {
	c, err := component.New("physiology:observable_state",
		component.WithDescription("Observable state of the human being (e.g., temperature, blood pressure etc)"),
		component.WithInputs(
			"time",
			"inspired_gas",
			"venous_blood",
			"brain_activity",
			"heart_cardiac_activation",
			"heart_rate",
			"pleural_pressure",
			"respiratory_rate",

			"lung_left_volume",
			"lung_left_flow",
			"lung_left_alveolar_pressure",
			"lung_left_exhaled_gas",
			"lung_left_alveolar_gas",

			"lung_right_volume",
			"lung_right_flow",
			"lung_right_alveolar_pressure",
			"lung_right_exhaled_gas",
			"lung_right_alveolar_gas",
		),
		component.WithOutputs(
			"is_alive",
			"inspired_gas",
			"venous_blood",
			"brain_activity",
			"brain_activity_trend",
			"heart_cardiac_activation",
			"heart_rate",
			"pleural_pressure",
			"respiratory_rate",
			"lung_left_volume",
			"lung_left_flow",
			"lung_left_alveolar_pressure",
			"lung_left_exhaled_gas",
			"lung_left_alveolar_gas",
			"lung_right_volume",
			"lung_right_flow",
			"lung_right_alveolar_pressure",
			"lung_right_exhaled_gas",
			"lung_right_alveolar_gas",
		),
		component.WithActivationFunc(helper.SequentialActivationFunc(
			handleBrainSignals,
			handleHeartSignals,
			handleDiaphragmSignals,
			handleLungSignals,
		)),
		component.WithInitialState(func(st component.State) {
			st.Set(LastBrainActivity, 0.0)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("physiology:observable_state: %w", err)
	}
	return c, nil
}

func handleBrainSignals(this *component.Component) error {
	if !this.InputByName("brain_activity").HasSignals() {
		return nil
	}

	this.OutputByName("is_alive").PutPayloads(true)

	// Calculate brain activity trend
	currentBrainActivity, err := helper.AsF64(this.InputByName("brain_activity").Signals().First())
	if err != nil {
		return err
	}
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

func handleDiaphragmSignals(this *component.Component) error {
	return helper.MultiForward(
		helper.PortPair{
			this.InputByName("pleural_pressure"),
			this.OutputByName("pleural_pressure"),
		},
		helper.PortPair{
			this.InputByName("respiratory_rate"),
			this.OutputByName("respiratory_rate"),
		})
}

func handleLungSignals(this *component.Component) error {
	return helper.MultiForward(
		helper.PortPair{
			this.InputByName("inspired_gas"),
			this.OutputByName("inspired_gas"),
		},
		helper.PortPair{
			this.InputByName("venous_blood"),
			this.OutputByName("venous_blood"),
		},
		helper.PortPair{
			this.InputByName("lung_left_volume"),
			this.OutputByName("lung_left_volume"),
		},
		helper.PortPair{
			this.InputByName("lung_left_flow"),
			this.OutputByName("lung_left_flow"),
		},
		helper.PortPair{
			this.InputByName("lung_left_alveolar_pressure"),
			this.OutputByName("lung_left_alveolar_pressure"),
		},
		helper.PortPair{
			this.InputByName("lung_left_exhaled_gas"),
			this.OutputByName("lung_left_exhaled_gas"),
		},
		helper.PortPair{
			this.InputByName("lung_left_alveolar_gas"),
			this.OutputByName("lung_left_alveolar_gas"),
		},
		helper.PortPair{
			this.InputByName("lung_right_volume"),
			this.OutputByName("lung_right_volume"),
		},
		helper.PortPair{
			this.InputByName("lung_right_flow"),
			this.OutputByName("lung_right_flow"),
		},
		helper.PortPair{
			this.InputByName("lung_right_alveolar_pressure"),
			this.OutputByName("lung_right_alveolar_pressure"),
		},
		helper.PortPair{
			this.InputByName("lung_right_exhaled_gas"),
			this.OutputByName("lung_right_exhaled_gas"),
		},
		helper.PortPair{
			this.InputByName("lung_right_alveolar_gas"),
			this.OutputByName("lung_right_alveolar_gas"),
		},
	)
}
