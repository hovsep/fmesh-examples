package organ

import (
	"context"
	"fmt"
	"math"

	"github.com/hovsep/fmesh-examples/life/autonomic"
	"github.com/hovsep/fmesh-examples/life/plugin/damage"
	"github.com/hovsep/fmesh-examples/life/plugin/perfusion"
	"github.com/hovsep/fmesh-examples/simulation/mathx"
	"github.com/hovsep/fmesh-examples/simulation/simtime"
	"github.com/hovsep/fmesh/component"
)

const (
	TidalRespiratoryRate = 12

	MinRespiratoryRate = 8
	MaxRespiratoryRate = 30

	BasePleuralPressure          = -5.0 // resting pleural pressure at FRC
	InspiratoryPressureAmplitude = 3.0  // peak swing during quiet breathing (−5 → −8 cmH₂O)

	// respiratoryRateJitter is how much breath-to-breath variation there is, as
	// a percentage. Real breathing is never metronomic.
	respiratoryRateJitter = 2.0

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

const DiaphragmO2PerMinute = 3.0

func GetDiaphragm() (*component.Component, error) {
	c, err := component.New("organ:diaphragm",
		component.WithDescription("Diaphragm (primary respiratory actuator)"),
		component.WithPlugins(
			damage.New(damage.Config{Organ: "diaphragm"}),
			// Quiet breathing is cheap; laboured breathing is not, and in
			// respiratory failure the muscle that breathes can end up consuming a
			// large share of the oxygen it is working to obtain.
			perfusion.New(perfusion.Config{Organ: "diaphragm", O2PerMinute: DiaphragmO2PerMinute}),
		),
		component.WithInputs("time", "autonomic_tone"),
		component.WithOutputs("pleural_pressure", "respiratory_rate"),
		// Not gated on damage.Working: a dead diaphragm must still publish a (constant)
		// pleural pressure, or the lungs wait forever for it and the mesh never
		// converges. Breathing stops because the pressure no longer oscillates.
		component.WithActivationFunc(
			component.Sequential(
				handleRespiratoryBias,
				oscillateBreathing,
			),
		),
		component.WithInitialState(func(state component.State) {
			state.Set(stateRate, TidalRespiratoryRate)
			state.Set(statePhase, 0.0)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("organ:diaphragm: %w", err)
	}
	return c, nil
}

func oscillateBreathing(_ context.Context, this *component.Component) error {
	if !this.InputByName("time").HasSignals() {
		return nil
	}

	// A failed diaphragm stops breathing: it holds a constant resting pleural
	// pressure (no inspiratory effort) so the lungs settle and stop ventilating,
	// rather than emitting nothing and leaving the lungs waiting for it.
	if damage.Failed(this) {
		if err := this.OutputByName("pleural_pressure").PutPayloads(BasePleuralPressure); err != nil {
			return err
		}
		return this.OutputByName("respiratory_rate").PutPayloads(0)
	}

	dt, err := simtime.TickDurationInSec(this.InputByName("time").Signals().First())
	if err != nil {
		return err
	}

	currentPhase := this.State().Get(statePhase).(float64)
	currentRate := this.State().Get(stateRate).(int)
	nextPhase := math.Mod(currentPhase+dt/(60.0/float64(currentRate)), 1.0)
	this.State().Set(statePhase, nextPhase)

	this.OutputByName("pleural_pressure").PutPayloads(diaphragmPressureWave(nextPhase))
	this.OutputByName("respiratory_rate").PutPayloads(this.State().Get(stateRate).(int))

	return nil
}

func handleRespiratoryBias(_ context.Context, this *component.Component) error {
	if !this.InputByName("autonomic_tone").HasSignals() {
		return nil
	}

	bias, err := autonomic.Bias(
		this.InputByName("autonomic_tone").Signals().First(),
		autonomic.Respiratory,
	)
	if err != nil {
		return err
	}

	// Nobody breathes at exactly twelve. A percent or two of jitter is the
	// difference between a body and a metronome, and it is small enough that
	// nothing downstream has to allow for it.
	this.State().Update(stateRate, func(v any) any {
		demanded := mathx.Lerp(MinRespiratoryRate, MaxRespiratoryRate, bias)
		return int(mathx.Clamp(mathx.Jitter(demanded, respiratoryRateJitter),
			MinRespiratoryRate, MaxRespiratoryRate))
	})

	return nil
}
