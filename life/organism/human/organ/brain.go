package organ

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/plugin/damage"
	"github.com/hovsep/fmesh-examples/life/plugin/perfusion"
	"github.com/hovsep/fmesh-examples/simulation/mathx"
	"github.com/hovsep/fmesh/component"
)

const (
	NeuralDrive       string = "neural_drive"
	NeuralDriveJitter        = 0.02
	MinNeuralDrive           = 0.0
	MaxNeuralDrive           = 1.0

	// 0.0 - 0.2 Sleep
	// 0.3 - 0.6 Baseline activity
	// 0.7 - 1.0 Stress, exercise, threat
	defaultNeuralDrive = 0.3

	// BrainO2PerMinute is the brain's resting oxygen demand in mL/min. It is a
	// fiftieth of the body by weight and takes a fifth of its oxygen, and it
	// cannot store any, which is why it is the first organ to fail when the
	// supply does.
	BrainO2PerMinute = 50.0

	// The brain tolerates a failing supply least of any organ, so it says so
	// rather than taking the default: its function starts to go while the blood
	// is still carrying what other tissues could live on.
	brainContentOnset = 15.0 // mL O₂ per dL of blood
	brainContentFail  = 8.0

	// Neuroglycopenia: confusion sets in in the 50s mg/dL and consciousness goes
	// around 30.
	glucoseFailLevel = 30.0 // mg/dL
	glucoseComfort   = 55.0
)

// @TODO: make brain to reduce\increase activity depending on inputs from other components (daylight, tireness, energy level, critical hungry etc)
func GetBrain() (*component.Component, error) {
	c, err := component.New("organ:brain",
		component.WithDescription("The Brain"),
		component.WithPlugins(
			damage.New(damage.Config{Organ: "brain"}),
			// The blood ports, the oxygen draw and the carbon dioxide it returns
			// all come from the plugin; the brain only says how much it needs.
			perfusion.New(perfusion.Config{
				Organ:        "brain",
				O2PerMinute:  BrainO2PerMinute,
				ContentOnset: brainContentOnset,
				ContentFail:  brainContentFail,
			}),
		),
		component.WithInputs("time"),          //@TODO while we strive to keep brain simple (brain must not directly command organs or say heart to beat, it only emits some driver signal), there must be many inputs from almost all organs and systems, right?
		component.WithOutputs("neural_drive"), // @TODO: let's think, maybe brain should emit more outputs (still keeping the golden rule: never directly command organs (double check if it is phisiologically correct))
		// When the brain fails it emits nothing; the body then reads no brain
		// activity and is declared dead.
		component.WithActivationFunc(damage.FlatlineWhenFailed(
			oscillateNeuralDrive,
		)),
		component.WithInitialState(func(state component.State) {
			state.Set(NeuralDrive, defaultNeuralDrive)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("organ:brain: %w", err)
	}
	return c, nil
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
		baseline = mathx.Clamp(mathx.Jitter(currentND.(float64), NeuralDriveJitter), MinNeuralDrive, MaxNeuralDrive)
		return baseline
	})

	// What the brain can actually do is capped by what the blood supplies: severe
	// hypoxia or hypoglycemia fades its drive toward zero (unconsciousness).
	drive := baseline * brainViability(this)
	return this.OutputByName("neural_drive").PutPayloads(drive)
}

// brainViability is 1 when the blood comfortably supplies the brain and falls to
// 0 as its oxygen or its sugar crosses into failure, whichever is worse.
//
// Oxygen is judged on what the blood is carrying rather than on its tension, so
// a brain starves when the body has bled even though the blood gas looks
// untouched. The glucose arm is separate because the brain, almost alone among
// tissues, cannot burn fat.
func brainViability(this *component.Component) float64 {
	glucose := perfusion.Read(this).Glucose
	glucoseFactor := mathx.Clamp((glucose-glucoseFailLevel)/(glucoseComfort-glucoseFailLevel), 0, 1)
	return min(perfusion.Sufficiency(this), glucoseFactor)
}
