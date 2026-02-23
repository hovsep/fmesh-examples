package physiology

import (
	"time"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// GetAutonomicCoordination ...
func GetAutonomicCoordination() *component.Component {
	return component.New("physiology:autonomic_coordination").
		WithDescription("Autonomic coordination system").
		AddInputs("time", "neural_drive").
		AddOutputs("autonomic_tone").
		WithActivationFunc(func(this *component.Component) error {
			if !this.InputByName("neural_drive").HasSignals() {
				this.Logger().Println("No signal from brain")
				return nil
			}

			this.OutputByName("autonomic_tone").PutPayloads(signal.NewGroup().Add(
				signal.New(0.0).AddLabel("type", "level").AddLabel("axis", "sympathetic"),
				signal.New(0.0).AddLabel("type", "level").AddLabel("axis", "parasympathetic"),
				signal.New(0.0).AddLabel("type", "level").AddLabel("axis", "noise"),
				signal.New(0.0).AddLabel("type", "level").AddLabel("axis", "gain"),
				signal.New(0.0).AddLabel("type", "level").AddLabel("axis", "regional_bias:cardiac"),
				signal.New(0.0).AddLabel("type", "level").AddLabel("axis", "regional_bias:vascular"),
				signal.New(0.0).AddLabel("type", "level").AddLabel("axis", "regional_bias:respiratory"),
				signal.New(0.0).AddLabel("type", "level").AddLabel("axis", "regional_bias:gi"),
				signal.New(1*time.Nanosecond).AddLabel("type", "duration").AddLabel("axis", "latency"),
			))
			return nil
		})

	//TODO:
	//Get oscilation signal from brain and broadcast:
	//autonomic_tone = {
	//  sympathetic_level: 0.0–1.0,
	//  parasympathetic_level: 0.0–1.0,
	//  gain: 0.0–1.0,
	//  latency_ms: number,
	//  noise: 0.0–1.0,
	//  regional_bias: { ... }
	//}
	//1. sympathetic_level: 0.0–1.0
	//
	//Meaning: Strength of the “fight or flight” bias from the brain to the body.
	//
	//Low (0.0–0.2): Relaxed, minimal stress. Body is conserving energy.
	//
	//Medium (0.3–0.6): Normal baseline tone. Organs maintain basic function.
	//
	//High (0.7–1.0): Stress, exercise, emergency. Increases heart rate, constricts vessels, dilates lungs, reduces gut activity.
	//
	//Simulation role: Organ nodes use this as a multiplier to modulate their output (e.g., heart rate, blood pressure).
	//
	//2. parasympathetic_level: 0.0–1.0
	//
	//Meaning: Strength of the “rest and digest” bias.
	//
	//Low (0.0–0.2): Parasympathetic drive minimal, could indicate stress, brain damage, or critical illness.
	//
	//Medium (0.3–0.6): Normal baseline, maintains digestion, resting heart rate.
	//
	//High (0.7–1.0): Deep relaxation or sleep. Heart rate drops, gut motility increases, secretions increase.
	//
	//Simulation role: Works in combination with sympathetic_level to determine organ response; not strictly the inverse of sympathetic.
	//
	//3. gain: 0.0–1.0
	//
	//Meaning: Overall strength or integrity of the signal. Represents how “healthy” the autonomic output is.
	//
	//1.0: Full, healthy brain output. Organs respond fully.
	//
	//<1.0: Reduced signal due to brain damage, hypoxia, trauma, or other systemic issues.
	//
	//0.0: No effective signal; organs cannot maintain coordinated function.
	//
	//Simulation role: Multiply sympathetic_level and parasympathetic_level by gain before organs respond. Key for emergent death/failure.
	//
	//4. latency_ms: number
	//
	//Meaning: Transmission delay from brain to autonomic physiology component.
	//
	//Low latency (5–20 ms): Healthy neural transmission.
	//
	//High latency (>50–100 ms): Neural dysfunction, ischemia, or partial failure.
	//
	//Simulation role: Delayed signal can produce desynchronization between organs (e.g., arrhythmias, unstable blood pressure). Optional but adds realism.
	//
	//5. noise: 0.0–1.0
	//
	//Meaning: Variability in the signal; simulates natural fluctuations in neural output.
	//
	//0.0: Perfectly rigid, unrealistically stable signal.
	//
	//0.1–0.3: Healthy variability (normal heart rate and blood pressure oscillations).
	//
	//>0.5: Dysautonomia or pathological instability.
	//
	//Simulation role: Add to sympathetic_level / parasympathetic_level or the oscillation to produce realistic variations; makes emergent organ behaviors natural.
	//
	//6. regional_bias: { ... }
	//
	//Meaning: Fine-grained modulation of specific organ systems. Each organ can have its own bias from the brain.

}
