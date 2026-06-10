package da

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/life/helper"
	. "github.com/hovsep/fmesh-examples/life/unit"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const MetabolicCO2Rate = 200.0 * Milliliter / 60.0 // mL CO2 per second at rest

// GetBloodSystem returns the blood system component
func GetBloodSystem() (*component.Component, error) {
	c, err := component.New("da:blood_system",
		component.WithDescription("Blood system"),
		component.WithInputs(
			"time",
			"alveolar_gas",
		),
		component.WithOutputs(
			"venous_co2",
		),
		component.WithActivationFunc(func(this *component.Component) error {
			if !this.Inputs().ByNames("time", "alveolar_gas").AllHaveSignals() {
				return component.ErrWaitingForInputsKeep
			}

			dt, err := helper.TickDurationInSec(this.InputByName("time").Signals().First())
			if err != nil {
				return err
			}

			co2Vol := MetabolicCO2Rate * dt // mL of CO2 per tick

			this.OutputByName("venous_co2").PutSignals(
				signal.New("venous_co2").
					WithLabel("category", "gas").
					WithLabel("type", "venous").
					WithScalar("CO2_volume", co2Vol),
			)
			return nil
		}),
		component.WithInitialState(func(state component.State) {
			state.Set("PO2", 0.0)
			state.Set("PCO2", 0.0)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("da:blood_system: %w", err)
	}
	return c, nil
}
