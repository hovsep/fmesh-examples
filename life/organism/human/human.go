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
		Debug:       false,
		CyclesLimit: 1000,
		TimeLimit:   5 * time.Second,
	})

	components := getComponents()
	// Add components to the mesh
	components.ForEach(func(c *component.Component) error {
		mesh.AddComponents(c)
		return mesh.ChainableErr()
	})

	// Do the wiring
	wireBrain(components)
	wireHeart(components)
	wireAutotomicCoordination(components)
	wireDiaphragm(components)
	wireRespiratoryBoundary(components)
	wireLungs(components)

	err := internal.HandleGraphFlag(mesh, true)
	if err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}

	return mesh
}

func wireBrain(components *component.Collection) {
	components.ByName("organ:brain").
		OutputByName("neural_drive").
		PipeTo(
			// Brain drives the autonomic coordination system
			components.ByName("physiology:autonomic_coordination").InputByName("neural_drive"),

			// Brain activity is observable
			components.ByName("physiology:observable_state").InputByName("brain_activity"),
		)
}

func wireAutotomicCoordination(components *component.Collection) {
	components.ByName("physiology:autonomic_coordination").OutputByName("autonomic_tone").PipeTo(
		// Affect the heart (cardiac bias)
		components.ByName("organ:heart").InputByName("autonomic_tone"),

		// Affect the diaphragm (respiratory bias)
		components.ByName("organ:diaphragm").InputByName("autonomic_tone"),
	)
}

func wireHeart(components *component.Collection) {
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
}

func wireDiaphragm(components *component.Collection) {
	components.ByName("organ:diaphragm").
		OutputByName("pleural_pressure").
		PipeTo(
			// Pleural pressure is observable
			components.ByName("physiology:observable_state").InputByName("pleural_pressure"),

			// And it drives the lungs
			components.ByName("organ:lung_left").InputByName("pleural_pressure"),
			components.ByName("organ:lung_right").InputByName("pleural_pressure"),
		)

	components.ByName("organ:diaphragm").
		OutputByName("respiratory_rate").
		PipeTo(
			// Respiratory rate is observable
			components.ByName("physiology:observable_state").InputByName("respiratory_rate"),
		)
}

func wireRespiratoryBoundary(components *component.Collection) {
	// Air flows from respiratory system to lungs
	components.ByName("boundary:respiratory").OutputByName("inspired_gas").PipeTo(
		components.ByName("organ:lung_left").InputByName("inspired_gas"),
		components.ByName("organ:lung_right").InputByName("inspired_gas"),
	)
}

func wireLungs(components *component.Collection) {
	// Lung flow is observable
	components.ByName("organ:lung_left").OutputByName("flow").PipeTo(
		components.ByName("physiology:observable_state").InputByName("lung_left_flow"),
	)

	components.ByName("organ:lung_right").OutputByName("flow").PipeTo(
		components.ByName("physiology:observable_state").InputByName("lung_right_flow"),
	)
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
			"lung_left_flow",
			"lung_right_flow",
		).
		WithActivationFunc(helper.Pipeline(
			getHabibatToHumanAF(mesh),
			getRunHumanAF(mesh),
			getHumanToObservableStateAF(mesh),
		),
		)
}

func getHabibatToHumanAF(mesh *fmesh.FMesh) component.ActivationFunc {
	return func(this *component.Component) error {
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
				this.InputByName("habitat_time_tick"),
				mesh.ComponentByName("organ:lung_left").InputByName("time"),
			},
			helper.PortPair{
				this.InputByName("habitat_time_tick"),
				mesh.ComponentByName("organ:lung_right").InputByName("time"),
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
		return nil
	}
}

func getRunHumanAF(mesh *fmesh.FMesh) component.ActivationFunc {
	return func(this *component.Component) error {
		_, err := mesh.Run()
		if err != nil {
			return fmt.Errorf("failed to run human mesh: %w", err)
		}
		return nil
	}
}

func getHumanToObservableStateAF(mesh *fmesh.FMesh) component.ActivationFunc {

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
				humanObservableState.OutputByName("lung_left_flow"),
				this.OutputByName("lung_left_flow"),
			},
			helper.PortPair{
				humanObservableState.OutputByName("lung_right_flow"),
				this.OutputByName("lung_right_flow"),
			},
		)
		if err != nil {
			return fmt.Errorf("failed to forward signals from human mesh: %w", err)
		}
		return nil
	}
}
