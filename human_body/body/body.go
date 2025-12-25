package body

import (
	"fmt"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/human_body/body/controller"
	"github.com/hovsep/fmesh-examples/human_body/body/organ"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
)

const (
	meshName      = "body_mesh"
	componentName = "body"
)

// getMesh builds the mesh that simulates the human body
func getMesh() *fmesh.FMesh {
	mesh := fmesh.NewWithConfig(meshName, &fmesh.Config{
		CyclesLimit: 0,
		TimeLimit:   10 * time.Second,
	})

	getBodyComponents().ForEach(func(c *component.Component) error {
		return mesh.AddComponents(c).ChainableErr()
	})

	return mesh
}

func getBodyComponents() *component.Collection {

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

// GetComponent wraps the body mesh into a fmesh component
func GetComponent() *component.Component {
	mesh := getMesh()

	return component.New(componentName).
		WithDescription("Human body component").
		AttachInputPorts(
			port.NewInput("time").
				WithDescription("Time signal").
				AddLabel("@autopipe-category", "env-factor").
				AddLabel("@autopipe-component", "time").
				AddLabel("@autopipe-port", "tick"),

			port.NewInput("env_temperature").
				WithDescription("Ambient temperature in Celsius degrees").
				AddLabel("@autopipe-category", "env-factor").
				AddLabel("@autopipe-component", "temperature").
				AddLabel("@autopipe-port", "current_temperature"),

			port.NewInput("uvi").
				WithDescription("Ultraviolet index").
				AddLabel("@autopipe-category", "env-factor").
				AddLabel("@autopipe-component", "sun").
				AddLabel("@autopipe-port", "uvi"),

			port.NewInput("lux").
				WithDescription("Illuminance in lux").
				AddLabel("@autopipe-category", "env-factor").
				AddLabel("@autopipe-component", "sun").
				AddLabel("@autopipe-port", "lux"),

			port.NewInput("air_humidity").
				WithDescription("Air humidity").
				AddLabel("@autopipe-category", "env-factor").
				AddLabel("@autopipe-component", "air").
				AddLabel("@autopipe-port", "humidity"),

			port.NewInput("air_composition").
				WithDescription("Air composition").
				AddLabel("@autopipe-category", "env-factor").
				AddLabel("@autopipe-component", "air").
				AddLabel("@autopipe-port", "composition"),
		).
		AddOutputs(). // Probably body state and maybe impact to env
		WithActivationFunc(func(this *component.Component) error {
			// read body inputs

			// pass body inputs into body mesh (route inputs to respective organs or central router)
			_, err := mesh.Run()

			if err != nil {
				return fmt.Errorf("failed to run %s: %w", mesh.Name(), err)
			}

			// extract body outputs from body mesh

			// put mesh outputs into component outputs

			return nil
		})
}
