package human

import (
	"fmt"
	"time"

	"github.com/hovsep/fmesh"
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
	mesh := fmesh.NewWithConfig(meshName, &fmesh.Config{
		// Guardrail: do not let human mesh to run forever
		CyclesLimit: 1000,
		TimeLimit:   10 * time.Second,
	})

	getComponents().ForEach(func(c *component.Component) error {
		return mesh.AddComponents(c).ChainableErr()
	})

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
			controller.GetPhysicalStress(),
			controller.GetMentalStress(),

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

// New wraps the human mesh wrapped into an FMesh component (so it can be injected into habitat mesh)
func New(name string) *component.Component {
	mesh := getMesh()

	return component.New("human-"+name).
		WithDescription("A human being").
		AddInputs(
			"habitat_time_tick",
			"habitat_sun2_uvi",
			"habitat_sun_lux",
			"habitat_air_temperature",
			"habitat_air_humidity",
			"habitat_air_composition",
		).
		AddOutputs(). // Probably human state, NO IMPACT TO HABITAT
		WithActivationFunc(func(this *component.Component) error {
			// read signals from habitat

			// route habitat signals to respective organs or central router
			_, err := mesh.Run()

			if err != nil {
				return fmt.Errorf("failed to run %s: %w", mesh.Name(), err)
			}

			// extract body outputs from body mesh

			// put mesh outputs into component outputs

			return nil
		})
}
