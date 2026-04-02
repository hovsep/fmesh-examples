package organ

import (
	"math"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/component"
)

const (
	TidalRespirationRate         = 12 * PerMinute
	minRespRate          float64 = 8 * PerMinute
	maxRespRate          float64 = 30 * PerMinute

	basePleuralPressure          float64 = -5 * CmH2O
	inspiratoryPressureAmplitude float64 = 8 * CmH2O
)

// diaphragmPressureWave defines breathing-driven pleural pressure
func diaphragmPressureWave(phase float64) float64 {
	var effort float64

	// Inhale (active contraction → more negative pressure)
	if phase < 0.4 {
		x := phase / 0.4
		effort = math.Sin(x * math.Pi / 2) // smooth 0 → 1
	} else {
		// Exhale (passive relaxation → return to baseline)
		x := (phase - 0.4) / 0.6
		effort = math.Exp(-3 * x)
	}

	return basePleuralPressure - inspiratoryPressureAmplitude*effort
}

// GetDiaphragm returns diaphragm component
func GetDiaphragm() *component.Component {
	return component.New("organ:diaphragm").
		WithDescription("Diaphragm (respiratory actuator)").
		WithInitialState(func(state component.State) {
			state.Set(common.Rate, TidalRespirationRate) // breaths per minute
			state.Set(common.Phase, 0.0)                 // breathing cycle phase
		}).
		AddInputs("time", "autonomic_tone").
		AddOutputs("pleural_pressure", "respiratory_rate").
		WithActivationFunc(
			helper.Pipeline(
				oscillateBreathing,
				handleRespiratoryBias,
			),
		)
}

func oscillateBreathing(this *component.Component) error {
	if !this.InputByName("time").HasSignals() {
		return nil
	}

	dt, err := helper.TickDurationInSec(this.InputByName("time").Signals().First())
	if err != nil {
		return err
	}

	var nextPhase float64

	this.State().Update(common.Phase, func(old any) any {
		currentPhase := old.(float64)
		currentRate := this.State().Get(common.Rate).(int)

		phaseStep := dt / (60.0 / float64(currentRate))
		nextPhase = math.Mod(currentPhase+phaseStep, 1.0)

		return nextPhase
	})

	pressure := diaphragmPressureWave(nextPhase)

	this.OutputByName("pleural_pressure").PutPayloads(pressure)

	return nil
}

func handleRespiratoryBias(this *component.Component) error {
	if !this.InputByName("autonomic_tone").HasSignals() {
		return nil
	}

	bias, err := helper.GetBias(
		this.InputByName("autonomic_tone").Signals().First(),
		common.Respiratory,
	)
	if err != nil {
		return err
	}

	this.State().Update(common.Rate, func(v any) any {
		return int(helper.Lerp(minRespRate, maxRespRate, bias))
	})

	this.OutputByName("respiratory_rate").PutPayloads(this.State().Get(common.Rate).(int))

	return nil
}
