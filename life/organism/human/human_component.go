package human

import (
	"fmt"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh/component"
)

// New returns a new human as a component (for simplicity we skip a clothing insulation factor, so the human being is naked)
func New(name string) *component.Component {
	mesh := getHumanMesh()

	return component.New("human-"+name).
		WithDescription("A human being").
		AddLabel("role", "organism").
		AddLabel("genus", "homo").
		AddLabel("species", "sapiens").
		AddInputs(
			"habitat_time_tick",
			"habitat_gas_environmental_gas",
		).
		AddOutputs(
			"is_alive",
			"brain_activity",
			"brain_activity_trend",
			"body_temperature",
			"heart_cardiac_activation",
			"heart_rate",
			"pleural_pressure",
			"respiratory_rate",
			"lung_left_volume",
			"lung_left_flow",
			"lung_left_alveolar_pressure",
			"lung_left_exhaled_gas",
			"lung_right_volume",
			"lung_right_flow",
			"lung_right_alveolar_pressure",
			"lung_right_exhaled_gas",
		).
		WithActivationFunc(helper.Pipeline(
			sense(mesh),
			act(mesh),
			feedback(mesh),
		))
}

// Sense activation function
// In this phase a human component receives inputs from the environment
func sense(mesh *fmesh.FMesh) component.ActivationFunc {
	return func(this *component.Component) error {
		if !this.Inputs().ByNames("habitat_time_tick", "habitat_gas_environmental_gas").AllHaveSignals() {
			return component.ErrWaitingForInputsKeep
		}
		respiratory := mesh.ComponentByName("boundary:respiratory")
		err := helper.MultiForward(
			// Time effect
			helper.PortPair{
				this.InputByName("habitat_time_tick"),
				mesh.ComponentByName("organ:brain").InputByName("time"),
			},
			helper.PortPair{
				this.InputByName("habitat_time_tick"),
				mesh.ComponentByName("organ:heart").InputByName("time"),
			},
			helper.PortPair{
				this.InputByName("habitat_time_tick"),
				respiratory.InputByName("time"),
			},
			helper.PortPair{
				this.InputByName("habitat_time_tick"),
				mesh.ComponentByName("organ:diaphragm").InputByName("time"),
			},
			helper.PortPair{
				this.InputByName("habitat_time_tick"),
				mesh.ComponentByName("organ:lung_left").InputByName("time"),
			},
			helper.PortPair{
				this.InputByName("habitat_time_tick"),
				mesh.ComponentByName("organ:lung_right").InputByName("time"),
			},

			// Gas effect
			helper.PortPair{
				this.InputByName("habitat_gas_environmental_gas"),
				respiratory.InputByName("environmental_gas"),
			})
		if err != nil {
			return fmt.Errorf("failed to forward signals into human mesh: %w", err)
		}
		return nil
	}
}

// Act activation function
// In this phase a human component runs inner mesh thus activating all organs and systems
func act(mesh *fmesh.FMesh) component.ActivationFunc {
	return func(this *component.Component) error {
		_, err := mesh.Run()
		if err != nil {
			return fmt.Errorf("failed to run human mesh: %w", err)
		}
		return nil
	}
}

// Feedback activation function
// In this phase a human component propagates outputs from the inner mesh to the human component
func feedback(mesh *fmesh.FMesh) component.ActivationFunc {

	return func(this *component.Component) error {

		humanObservableState := mesh.ComponentByName("physiology:observable_state")

		// Propagate signals from human mesh to the human component outputs
		err := helper.MultiForward(
			helper.PortPair{
				humanObservableState.OutputByName("is_alive"),
				this.OutputByName("is_alive"),
			},
			helper.PortPair{
				humanObservableState.OutputByName("brain_activity"),
				this.OutputByName("brain_activity"),
			},
			helper.PortPair{
				humanObservableState.OutputByName("brain_activity_trend"),
				this.OutputByName("brain_activity_trend"),
			},
			helper.PortPair{
				humanObservableState.OutputByName("heart_cardiac_activation"),
				this.OutputByName("heart_cardiac_activation"),
			},
			helper.PortPair{
				humanObservableState.OutputByName("heart_rate"),
				this.OutputByName("heart_rate"),
			},
			helper.PortPair{
				humanObservableState.OutputByName("pleural_pressure"),
				this.OutputByName("pleural_pressure"),
			},
			helper.PortPair{
				humanObservableState.OutputByName("respiratory_rate"),
				this.OutputByName("respiratory_rate"),
			},
			helper.PortPair{
				humanObservableState.OutputByName("lung_left_volume"),
				this.OutputByName("lung_left_volume"),
			},
			helper.PortPair{
				humanObservableState.OutputByName("lung_left_flow"),
				this.OutputByName("lung_left_flow"),
			},
			helper.PortPair{
				humanObservableState.OutputByName("lung_left_alveolar_pressure"),
				this.OutputByName("lung_left_alveolar_pressure"),
			},
			helper.PortPair{
				humanObservableState.OutputByName("lung_left_exhaled_gas"),
				this.OutputByName("lung_left_exhaled_gas"),
			},
			helper.PortPair{
				humanObservableState.OutputByName("lung_right_volume"),
				this.OutputByName("lung_right_volume"),
			},
			helper.PortPair{
				humanObservableState.OutputByName("lung_right_flow"),
				this.OutputByName("lung_right_flow"),
			},
			helper.PortPair{
				humanObservableState.OutputByName("lung_right_alveolar_pressure"),
				this.OutputByName("lung_right_alveolar_pressure"),
			},
			helper.PortPair{
				humanObservableState.OutputByName("lung_right_exhaled_gas"),
				this.OutputByName("lung_right_exhaled_gas"),
			},
		)
		if err != nil {
			return fmt.Errorf("failed to forward signals from human mesh: %w", err)
		}
		return nil
	}
}
