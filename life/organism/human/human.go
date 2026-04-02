package human

import (
	"fmt"
	"os"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/helper"
	"github.com/hovsep/fmesh-examples/life/organism/human/boundary"
	"github.com/hovsep/fmesh-examples/life/organism/human/controller"
	da "github.com/hovsep/fmesh-examples/life/organism/human/distributed_anatomy"
	"github.com/hovsep/fmesh-examples/life/organism/human/organ"
	"github.com/hovsep/fmesh-examples/life/organism/human/physiology"
	"github.com/hovsep/fmesh-examples/life/organism/human/regulation"
	"github.com/hovsep/fmesh/component"
)

const (
	meshName = "human_mesh"
)

// getMesh builds the mesh that simulates the human being
func getMesh() *fmesh.FMesh {
	// Create the mesh
	mesh := fmesh.NewWithConfig(meshName, &fmesh.Config{
		// Guardrail: do not let human mesh to run forever
		Debug:       false,
		CyclesLimit: 1000,
		TimeLimit:   60 * time.Second,
	})

	components := getComponents()
	// Add components to the mesh
	components.ForEach(func(c *component.Component) error {
		return mesh.AddComponents(c).ChainableErr()
	})

	// Do the wiring
	components.ByName("organ:brain").
		OutputByName("neural_drive").
		PipeTo(
			// Brain drives the autonomic coordination system
			components.ByName("physiology:autonomic_coordination").InputByName("neural_drive"),

			// Brain activity is observable
			components.ByName("physiology:observable_state").InputByName("brain_activity"),
		)

	components.ByName("organ:heart").
		OutputByName("cardiac_activation").
		PipeTo(
			// Heart activity is observable
			components.ByName("physiology:observable_state").InputByName("heart_cardiac_activation"),
		)

	components.ByName("organ:heart").
		OutputByName("rate").
		PipeTo(
			// Heart rate is observable
			components.ByName("physiology:observable_state").InputByName("heart_rate"),
		)

	components.ByName("physiology:autonomic_coordination").OutputByName("autonomic_tone").PipeTo(
		// Autonomic tone affects the heart
		components.ByName("organ:heart").InputByName("autonomic_tone"),
		components.ByName("organ:diaphragm").InputByName("autonomic_tone"),
	)

	components.ByName("organ:diaphragm").
		OutputByName("pleural_pressure").
		PipeTo(
			// Heart activity is observable
			components.ByName("physiology:observable_state").InputByName("pleural_pressure"),
		)

	components.ByName("organ:diaphragm").
		OutputByName("respiratory_rate").
		PipeTo(
			// Heart rate is observable
			components.ByName("physiology:observable_state").InputByName("respiratory_rate"),
		)

	components.ByName("boundary:respiratory").OutputByName("inspired_gas").PipeTo(
		components.ByName("organ:lung_left").InputByName("inspired_gas"),
		components.ByName("organ:lung_right").InputByName("inspired_gas"),
	)

	err := internal.HandleGraphFlag(mesh, true)
	if err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}

	return mesh
}

// getComponents returns the collection of human components (organs, systems, etc.)
func getComponents() *component.Collection {
	// @TODO:

	// other organs:
	// liver,
	// kidneys,

	// distributed anatomy:
	// immune system
	// nutritional/metabolic system
	// fluid balance

	// logical components:
	// internal physiological load
	// aggregated state (heart rate, breath, oxygen saturation, body temp, systemic stress index, fatigue, blood pH, blood volume, pain level, inflammation level)

	return component.NewCollection().
		Add(
			// Boundaries (interfaces between the environment and a human body)
			boundary.GetThermal(),
			boundary.GetMechanical(),
			boundary.GetRespiratory(),
			boundary.GetIngestion(),

			// Controllers (intention input from simulation operator, like eating food, drinking water or receiving emotional stimuli)
			controller.GetIntake(),
			controller.GetPhysical(),
			controller.GetMental(),
			controller.GetExcretion(),

			// Physiological systems
			physiology.GetAutonomicCoordination(),
			physiology.GetPhysiologicalLoad(),
			physiology.GetEndocrineAxis(),
			physiology.GetObservableState(),
			physiology.GetPhysiologicalState(),

			// Regulation systems
			regulation.GetHomeostasis(),

			// Organs
			organ.GetBrain(),
			organ.GetHeart(),
			organ.GetDiaphragm(),
			organ.GetLung(common.Left),
			organ.GetLung(common.Right),

			// Distributed anatomy
			da.GetSkin(),
			da.GetBloodSystem(),
			da.GetMuscularSystem(),
			da.GetNervousSystem(),
			da.GetGITract(),
		)
}

// New returns a new human as a component (for simplicity we skip a clothing insulation factor, so the human being is naked)
func New(name string) *component.Component {
	mesh := getMesh()

	return component.New("human-"+name).
		WithDescription("A human being").
		AddLabel("role", "organism").
		AddLabel("genus", "homo").
		AddLabel("species", "sapiens").
		AddInputs(
			"habitat_time_tick",
			"habitat_gas_temperature",
			"habitat_gas_humidity",
			"habitat_gas_composition",
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
		).
		WithActivationFunc(func(this *component.Component) error {
			// read signals from habitat

			// route habitat signals to respective organs or central router
			respiratory := mesh.ComponentByName("boundary:respiratory")
			err := helper.MultiForward(
				// Time to organs
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
					mesh.ComponentByName("organ:diaphragm").InputByName("time"),
				},

				helper.PortPair{
					this.InputByName("habitat_gas_temperature"),
					respiratory.InputByName("gas_temperature"),
				},
				helper.PortPair{
					this.InputByName("habitat_gas_humidity"),
					respiratory.InputByName("gas_humidity"),
				},
				helper.PortPair{
					this.InputByName("habitat_gas_composition"),
					respiratory.InputByName("gas_composition"),
				})
			if err != nil {
				return fmt.Errorf("failed to forward signals into human mesh: %w", err)
			}

			_, err = mesh.Run()

			if err != nil {
				return fmt.Errorf("failed to run %s: %w", mesh.Name(), err)
			}

			humanObservableState := mesh.ComponentByName("physiology:observable_state")

			// Propagate signals from human mesh to the human component outputs
			err = helper.MultiForward(
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
			)
			if err != nil {
				return fmt.Errorf("failed to forward signals from human mesh: %w", err)
			}
			return nil
		})
}
