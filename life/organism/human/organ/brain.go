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

	// Metabolism: the brain is the body's hungriest organ for its size, taking
	// about a fifth of resting oxygen use, and returns carbon dioxide to the
	// blood. Rates are in mmHg/s of arterial tension.
	BrainO2Consumption = 0.35 * MmHgPerSecond
	BrainCO2Production = 0.05 * MmHgPerSecond

	// The brain is the first organ to suffer when the blood cannot supply it.
	// Below these tensions its drive fades toward zero (unconsciousness); above
	// the comfortable ones it is unaffected.
	lastBloodO2      common.State = "last_blood_o2"
	lastBloodGlucose common.State = "last_blood_glucose"

	// o2ComfortLevel is the PaO₂ at which saturation is still ~90%, the point at
	// which oxygen would be given clinically; o2FailLevel is roughly where
	// saturation has fallen far enough to cost consciousness.
	o2FailLevel    = 25.0 * MmHg
	o2ComfortLevel = 60.0 * MmHg

	// Neuroglycopenia: confusion sets in in the 50s mg/dL and consciousness goes
	// around 30.
	glucoseFailLevel = 30.0 // mg/dL
	glucoseComfort   = 55.0
)

func GetBrain() (*component.Component, error) {
	c, err := component.New("organ:brain",
		component.WithDescription("The Brain"),
		component.WithPlugins(damage.New(damage.Config{Organ: "brain"})),
		component.WithInputs("time", "blood"),
		component.WithOutputs("neural_drive", "blood"),
		// When the brain fails it emits nothing; the body then reads no brain
		// activity and is declared dead.
		component.WithActivationFunc(damage.FlatlineWhenFailed(
			senseBlood,
			oscillateNeuralDrive,
			emitBrainMetabolism,
		)),
		component.WithInitialState(func(state component.State) {
			state.Set(NeuralDrive, defaultNeuralDrive)
			state.Set(lastBloodO2, da.NormalPaO2)
			state.Set(lastBloodGlucose, da.DefaultGlucoseLevel)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("organ:brain: %w", err)
	}
	return c, nil
}

// senseBlood latches the blood O2 and glucose the brain is being supplied,
// since the blood signal arrives on its own mesh cycle.
func senseBlood(this *component.Component) error {
	in := this.InputByName("blood")
	if !in.HasSignals() {
		return nil
	}
	if sig := in.Signals().First(); sig != nil {
		this.State().Set(lastBloodO2, sig.Scalars().ValueOrDefault("PaO2", da.NormalPaO2))
		this.State().Set(lastBloodGlucose, sig.Scalars().ValueOrDefault("glucose_level", da.DefaultGlucoseLevel))
	}
	return nil
}

func oscillateNeuralDrive(this *component.Component) error {
	// Only advance on a time tick; blood-only activations (from the shared bus) are ignored.
	if !this.InputByName("time").HasSignals() {
		return nil
	}

	// The baseline drive is the usual jittered random walk; it is stored so it
	// recovers once the blood is restored.
	var baseline float64
	this.State().Update(NeuralDrive, func(currentND any) any {
		baseline = helper.Clamp(helper.Jitter(currentND.(float64), NeuralDriveJitter), MinNeuralDrive, MaxNeuralDrive)
		return baseline
	})

	// What the brain can actually do is capped by what the blood supplies: severe
	// hypoxia or hypoglycemia fades its drive toward zero (unconsciousness).
	drive := baseline * brainViability(this)
	return this.OutputByName("neural_drive").PutPayloads(drive)
}

// brainViability is 1 when the blood comfortably supplies the brain and falls to
// 0 as O2 or glucose crosses into failure, whichever is worse.
func brainViability(this *component.Component) float64 {
	o2 := this.State().Get(lastBloodO2).(float64)
	glucose := this.State().Get(lastBloodGlucose).(float64)

	o2Factor := helper.Clamp((o2-o2FailLevel)/(o2ComfortLevel-o2FailLevel), 0, 1)
	glucoseFactor := helper.Clamp((glucose-glucoseFailLevel)/(glucoseComfort-glucoseFailLevel), 0, 1)
	return min(o2Factor, glucoseFactor)
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
