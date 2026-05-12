package human

import (
	"fmt"
	"os"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/organism/human/boundary"
	"github.com/hovsep/fmesh-examples/life/organism/human/organ"
	"github.com/hovsep/fmesh-examples/life/organism/human/physiology"
	"github.com/hovsep/fmesh/component"
)

const (
	meshName = "human_mesh"
)

// getHumanMesh builds the mesh that simulates the human being
func getHumanMesh() *fmesh.FMesh {
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

	err := internal.HandleGraphFlag(mesh, false)
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
	components.ByName("organ:lung_left").OutputByName("volume").PipeTo(
		components.ByName("physiology:observable_state").InputByName("lung_left_volume"),
	)
	components.ByName("organ:lung_left").OutputByName("flow").PipeTo(
		components.ByName("physiology:observable_state").InputByName("lung_left_flow"),
	)
	components.ByName("organ:lung_left").OutputByName("alveolar_pressure").PipeTo(
		components.ByName("physiology:observable_state").InputByName("lung_left_alveolar_pressure"),
	)
	components.ByName("organ:lung_left").OutputByName("exhaled_gas").PipeTo(
		components.ByName("physiology:observable_state").InputByName("lung_left_exhaled_gas"),
	)

	components.ByName("organ:lung_right").OutputByName("volume").PipeTo(
		components.ByName("physiology:observable_state").InputByName("lung_right_volume"),
	)
	components.ByName("organ:lung_right").OutputByName("flow").PipeTo(
		components.ByName("physiology:observable_state").InputByName("lung_right_flow"),
	)
	components.ByName("organ:lung_right").OutputByName("alveolar_pressure").PipeTo(
		components.ByName("physiology:observable_state").InputByName("lung_right_alveolar_pressure"),
	)
	components.ByName("organ:lung_right").OutputByName("exhaled_gas").PipeTo(
		components.ByName("physiology:observable_state").InputByName("lung_right_exhaled_gas"),
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
			//boundary.GetThermal(),
			//boundary.GetMechanical(),
			boundary.GetRespiratory(),
			//boundary.GetIngestion(),

			// Controllers (intention input from simulation operator, like eating food, drinking water or receiving emotional stimuli)
			//controller.GetIntake(),
			//controller.GetPhysical(),
			//controller.GetMental(),
			//controller.GetExcretion(),

			// Physiological systems
			physiology.GetAutonomicCoordination(),
			//physiology.GetPhysiologicalLoad(),
			//physiology.GetEndocrineAxis(),
			physiology.GetObservableState(),
			//physiology.GetPhysiologicalState(),

			// Regulation systems
			//regulation.GetHomeostasis(),

			// Organs
			organ.GetBrain(),
			organ.GetHeart(),
			organ.GetDiaphragm(),
			organ.GetLung(common.Left),
			organ.GetLung(common.Right),

			// Distributed anatomy
			//da.GetSkin(),
			//da.GetBloodSystem(),
			//da.GetMuscularSystem(),
			//da.GetNervousSystem(),
			//da.GetGITract(),
		)
}
