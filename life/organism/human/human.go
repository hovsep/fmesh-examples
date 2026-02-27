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
			components.ByName("physiology:autonomic_coordination").InputByName("neural_drive"),
			components.ByName("physiology:observable_state").InputByName("brain_activity"),
		)

	//DEBUG_START
	components.ByName("physiology:autonomic_coordination").SetupHooks(func(h *component.Hooks) {
		h.AfterActivation(func(ctx *component.ActivationContext) error {
			tone := ctx.Component.OutputByName("autonomic_tone").Signals().First()
			if tone == nil {
				return fmt.Errorf("autonomic tone signal not found")
			}
			//sym, paraSym, noise, gain, cardiacB, vascularB, respiratoryB, giB := physiology.UnpackAutonomicTone(tone)

			//ctx.Component.Logger().Printf("autonomic tone: %v, %v, %v, %v, %v, %v, %v, %v", sym, paraSym, noise, gain, cardiacB, vascularB, respiratoryB, giB)
			return nil
		})
	})
	//DEBUG_END

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
			"brain_activity",
			"brain_activity_trend",
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
			)
			if err != nil {
				return fmt.Errorf("failed to forward signals from human mesh: %w", err)
			}
			return nil
		})
}
