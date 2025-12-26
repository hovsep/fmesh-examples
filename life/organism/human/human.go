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
	meshName      = "human_mesh"
	componentName = "human"
)

// getMesh builds the mesh that simulates the human being
func getMesh() *fmesh.FMesh {
	mesh := fmesh.NewWithConfig(meshName, &fmesh.Config{
		CyclesLimit: 0,
		TimeLimit:   10 * time.Second,
	})

	getHumanComponents().ForEach(func(c *component.Component) error {
		return mesh.AddComponents(c).ChainableErr()
	})

	return mesh
}

func getHumanComponents() *component.Collection {

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

// GetComponent wraps the human mesh into an FMesh component (so it can be injected into habitat mesh)
func GetComponent() *component.Component {
	mesh := getMesh()

	return component.New(componentName).
		WithDescription("Human being component (a facade for the habibat)").
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
				AddLabel("@autopipe-port", "current_temperature"),

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
		AddOutputs(). // Probably human state and maybe impact to habitat
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
