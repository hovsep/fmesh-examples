package human

import (
	"fmt"
	"time"

	"github.com/hovsep/fmesh"
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
		CyclesLimit: 1000,
		TimeLimit:   10 * time.Second,
	})

	components := getComponents()
	// Add components to the mesh
	components.ForEach(func(c *component.Component) error {
		return mesh.AddComponents(c).ChainableErr()
	})

	// Do the wiring
	components.ByName("organ:brain").OutputByName("neural_drive").PipeTo(components.ByName("physiology:autonomic_coordination").InputByName("neural_drive"))
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
			"habitat_air_temperature",
			"habitat_air_humidity",
			"habitat_air_composition",
		).
		AddOutputs(
			"is_alive",
			"body_temperature",
			"heartbeat",
		).
		WithInitialState(func(state component.State) {

		}).
		WithActivationFunc(func(this *component.Component) error {
			// read signals from habitat

			// route habitat signals to respective organs or central router

			respiratory := mesh.ComponentByName("boundary:respiratory")
			err := helper.MultiForward(
				helper.PortPair{
					this.InputByName("habitat_time_tick"),
					mesh.ComponentByName("organ:brain").InputByName("time"),
				},
				helper.PortPair{
					this.InputByName("habitat_air_temperature"),
					respiratory.InputByName("air_temperature"),
				},
				helper.PortPair{
					this.InputByName("habitat_air_humidity"),
					respiratory.InputByName("air_humidity"),
				},
				helper.PortPair{
					this.InputByName("habitat_air_composition"),
					respiratory.InputByName("air_composition"),
				})
			if err != nil {
				return fmt.Errorf("failed to forward air signals: %w", err)
			}

			_, err = mesh.Run()

			if err != nil {
				return fmt.Errorf("failed to run %s: %w", mesh.Name(), err)
			}

			// extract body outputs from body mesh

			// put mesh outputs into component outputs

			this.OutputByName("is_alive").PutPayloads(false)

			return nil
		})
}
