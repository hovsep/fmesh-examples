package human

import (
	"fmt"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/life/organism/human/controller"
	"github.com/hovsep/fmesh-examples/life/organism/human/organ"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
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

	//@todo: all habitat signals must go to "interface" organ, e.g. air->lungs and never air->blood system
	// interfaces:
	// human_time (possibility to run inner mesh for 3 or more phases: sense, act, aggregate)
	// respiratory_interface
	// GL tract
	// skin

	// @TODO:

	// other organs:
	// lungs,
	// liver,
	// kidneys,
	// GL tract,

	// distributed anatomy:
	// skeletal system
	// cardiovascular system
	// muscular system
	// endocrine system
	// respiratory system
	// immune system
	// nutritional/metabolic system
	// autonomic nervous system
	// skin
	// fluid balance

	// logical components:

	// internal physiological load
	// aggregated state (heart rate, breath, oxygen saturation, body temp, systemic stress index, fatigue, blood pH, blood volume, pain level, inflammation level)

	return component.NewCollection().
		Add(
			organ.GetBrainComponent(),
			organ.GetHeartComponent(),

			controller.GetIntakeComponent(),
			controller.GetPhysicalStressComponent(),
			controller.GetMentalStressComponent(),
		)
}

// Build wraps the human mesh into an FMesh component (so it can be injected into habitat mesh)
func Build(name string) *component.Component {
	mesh := getMesh()

	return component.New("human-"+name).
		WithDescription("A human being").
		AttachInputPorts(
			port.NewInput("habitat_time").
				WithDescription("Time signal").
				AddLabel("@autopipe-category", "habitat-factor").
				AddLabel("@autopipe-component", "time").
				AddLabel("@autopipe-port", "tick"),

			port.NewInput("habitat_temperature").
				WithDescription("Ambient temperature in Celsius degrees").
				AddLabel("@autopipe-category", "habitat-factor").
				AddLabel("@autopipe-component", "temperature").
				AddLabel("@autopipe-port", "temperature"),

			port.NewInput("habitat_uvi").
				WithDescription("Ultraviolet index").
				AddLabel("@autopipe-category", "habitat-factor").
				AddLabel("@autopipe-component", "sun").
				AddLabel("@autopipe-port", "uvi"),

			port.NewInput("habitat_lux").
				WithDescription("Illuminance in lux").
				AddLabel("@autopipe-category", "habitat-factor").
				AddLabel("@autopipe-component", "sun").
				AddLabel("@autopipe-port", "lux"),

			port.NewInput("habitat_air_humidity").
				WithDescription("Air humidity").
				AddLabel("@autopipe-category", "habitat-factor").
				AddLabel("@autopipe-component", "air").
				AddLabel("@autopipe-port", "humidity"),

			port.NewInput("habitat_air_composition").
				WithDescription("Air composition").
				AddLabel("@autopipe-category", "habitat-factor").
				AddLabel("@autopipe-component", "air").
				AddLabel("@autopipe-port", "composition"),
		).
		AddOutputs(). // Probably human state, NO IMPACT TO HABIBAT
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
