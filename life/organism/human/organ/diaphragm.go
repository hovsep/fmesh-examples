package organ

import (
	"math"

	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/component"
)

const (
	TidalRespiratoryRate = 12 * PerMinute

	MinRespiratoryRate = 8 * PerMinute
	MaxRespiratoryRate = 30 * PerMinute

	BasePleuralPressure          = -5 * CmH2O // resting pleural pressure at FRC
	InspiratoryPressureAmplitude = 5 * CmH2O  // peak swing during quiet breathing

	inhaleFraction = 1.0 / 3.0 // I:E = 1:2
	exhaleDecay    = 5.0       // exp(-5) ≈ 0.007 residual — negligible pressure step at cycle restart
)

func diaphragmPressureWave(phase float64) float64 {
	var effort float64

	if phase < inhaleFraction {
		x := phase / inhaleFraction
		effort = math.Sin(x * math.Pi / 2)
	} else {
		x := (phase - inhaleFraction) / (1.0 - inhaleFraction)
		effort = math.Exp(-exhaleDecay * x)
	}

	return BasePleuralPressure - InspiratoryPressureAmplitude*effort
}

func GetDiaphragm() *component.Component {
	return component.New("organ:diaphragm").
		WithDescription("Diaphragm (primary respiratory actuator)").
		WithInitialState(func(state component.State) {
			state.Set(common.Rate, TidalRespiratoryRate)
			state.Set(common.Phase, 0.0)
		}).
		AddInputs("time", "autonomic_tone").
		AddOutputs("pleural_pressure", "respiratory_rate").
		WithActivationFunc(
			helper.Pipeline(
				handleRespiratoryBias, // update rate before advancing phase
				oscillateBreathing,
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

	currentPhase := this.State().Get(common.Phase).(float64)
	currentRate := this.State().Get(common.Rate).(int)
	nextPhase := math.Mod(currentPhase+dt/(60.0/float64(currentRate)), 1.0)
	this.State().Set(common.Phase, nextPhase)

	this.OutputByName("pleural_pressure").PutPayloads(diaphragmPressureWave(nextPhase))
	this.OutputByName("respiratory_rate").PutPayloads(this.State().Get(common.Rate).(int))

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
		return int(helper.Lerp(MinRespiratoryRate, MaxRespiratoryRate, bias))
	})

	return nil
}
