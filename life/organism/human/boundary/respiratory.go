package boundary

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/atmosphere"
	"github.com/hovsep/fmesh-examples/simulation/simtime"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// ScalarToxin is the damage dose carried on a substance load.
const ScalarToxin = "toxin"

// toxinPerPollutionPointPerSec converts foul air into injury.
//
// Calibrated so that a cigarette's worth of smoke -- two points of pollution for
// the seven or eight minutes one takes -- does a cigarette's worth of harm, and
// six hundred-odd of them fail a lung. The airway traps half of what comes in
// before it reaches the alveoli, which filterInspiredGas has always done and
// which is why the dose is taken after it.
const toxinPerPollutionPointPerSec = 3.4e-6

// cleanAirPollution is how dirty ordinary outdoor air already is. Only what a
// body breathes above this counts as an insult; otherwise every lung would be
// slowly failing from having been outdoors.
const cleanAirPollution = 0.4

func GetRespiratory() (*component.Component, error) {
	c, err := component.New("boundary:respiratory",
		component.WithDescription("Represents path from nose to trachea.Transforms environmental gas signals into chemical levels and lung input for circulation"),
		component.WithInputs(
			"time",
			"environmental_gas",
		),
		component.WithOutputs(
			"inspired_gas",
			// What was in the air, once it is in the body. Smoke is breathed, not
			// swallowed, so this is where its harm enters -- the same port an
			// allergen or a lungful of dust would use.
			"substance_load",
		),
		component.WithActivationFunc(component.Sequential(
			component.Pipeline([]string{"environmental_gas"}, "inspired_gas",
				filterInspiredGas, humidifyInspiredGas, warmUpInspiredGas),
			inhaleToxins,
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("boundary:respiratory: %w", err)
	}
	return c, nil
}

// inhaleToxins turns the dirt in the inspired air into a dose of harm.
func inhaleToxins(this *component.Component) error {
	tick := this.InputByName("time").Signals().First()
	if tick == nil {
		return nil
	}
	dt, err := simtime.TickDurationInSec(tick)
	if err != nil {
		return nil
	}

	inspired := this.OutputByName("inspired_gas").Signals().First()
	if inspired == nil {
		return nil
	}
	pollution := inspired.Scalars().ValueOrDefault("composition:pollution", 0)

	excess := pollution - cleanAirPollution/2 // the airway already took half
	if excess <= 0 {
		return nil
	}

	dose := excess * toxinPerPollutionPointPerSec * dt
	return this.OutputByName("substance_load").PutSignals(
		signal.New(dose).
			WithLabel("category", "inhaled").
			WithScalar(ScalarToxin, dose))
}

// Applies pollution reduction.
func filterInspiredGas(sigs *signal.Group) (*signal.Group, error) {
	result := sigs.MapIf(func(s *signal.Signal) bool {
		return s.Labels().ValueIs("category", "gas") && s.Labels().ValueIs("type", "air")
	}, func(airSignal *signal.Signal) *signal.Signal {
		return atmosphere.MapScalar(airSignal, "composition:pollution", func(p float64) float64 {
			return p * 0.5
		})
	})
	return result, nil
}

// Applies humidity increase.
func humidifyInspiredGas(sigs *signal.Group) (*signal.Group, error) {
	result := sigs.MapIf(func(s *signal.Signal) bool {
		return s.Labels().ValueIs("category", "gas") && s.Labels().ValueIs("type", "air")
	}, func(airSignal *signal.Signal) *signal.Signal {
		return atmosphere.MapScalar(airSignal, "humidity", func(h float64) float64 {
			return h * 1.1
		})
	})
	return result, nil
}

// Applies temperature increase.
func warmUpInspiredGas(sigs *signal.Group) (*signal.Group, error) {
	result := sigs.MapIf(func(s *signal.Signal) bool {
		return s.Labels().ValueIs("category", "gas") && s.Labels().ValueIs("type", "air")
	}, func(airSignal *signal.Signal) *signal.Signal {
		return atmosphere.MapScalar(airSignal, "temperature", func(t float64) float64 {
			return t + 0.2
		})
	})
	return result, nil
}
