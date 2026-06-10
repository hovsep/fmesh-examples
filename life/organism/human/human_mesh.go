package human

import (
	"fmt"
	"os"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/organism/human/boundary"
	"github.com/hovsep/fmesh-examples/life/organism/human/distributed_anatomy"
	"github.com/hovsep/fmesh-examples/life/organism/human/organ"
	"github.com/hovsep/fmesh-examples/life/organism/human/physiology"
	"github.com/hovsep/fmesh/component"
)

const (
	meshName = "human_mesh"
)

// getHumanMesh builds the mesh that simulates the human being
func getHumanMesh() (*fmesh.FMesh, error) {
	// Create the mesh
	mesh, err := fmesh.New(meshName,
		fmesh.WithCyclesLimit(1000),
		fmesh.WithTimeLimit(5*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create human mesh: %w", err)
	}

	components, err := getComponents()
	if err != nil {
		return nil, fmt.Errorf("getComponents: %w", err)
	}
	// Add components to the mesh
	if err := components.ForEach(func(c *component.Component) error {
		return mesh.AddComponents(c)
	}); err != nil {
		return nil, fmt.Errorf("failed to add components to human mesh: %w", err)
	}

	// Do the wiring
	if err := wireBrain(components); err != nil {
		return nil, fmt.Errorf("wireBrain: %w", err)
	}
	if err := wireHeart(components); err != nil {
		return nil, fmt.Errorf("wireHeart: %w", err)
	}
	if err := wireAutotomicCoordination(components); err != nil {
		return nil, fmt.Errorf("wireAutotomicCoordination: %w", err)
	}
	if err := wireDiaphragm(components); err != nil {
		return nil, fmt.Errorf("wireDiaphragm: %w", err)
	}
	if err := wireRespiratoryBoundary(components); err != nil {
		return nil, fmt.Errorf("wireRespiratoryBoundary: %w", err)
	}
	if err := wireLungs(components); err != nil {
		return nil, fmt.Errorf("wireLungs: %w", err)
	}
	if err := wireBloodSystem(components); err != nil {
		return nil, fmt.Errorf("wireBloodSystem: %w", err)
	}

	err = internal.HandleGraphFlag(mesh, false)
	if err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}

	return mesh, nil
}

func wireBrain(components *component.Collection) error {
	return components.ByName("organ:brain").
		OutputByName("neural_drive").
		PipeTo(
			// Brain drives the autonomic coordination system
			components.ByName("physiology:autonomic_coordination").InputByName("neural_drive"),

			// Brain activity is observable
			components.ByName("physiology:observable_state").InputByName("brain_activity"),
		)
}

func wireAutotomicCoordination(components *component.Collection) error {
	return components.ByName("physiology:autonomic_coordination").OutputByName("autonomic_tone").PipeTo(
		// Affect the heart (cardiac bias)
		components.ByName("organ:heart").InputByName("autonomic_tone"),

		// Affect the diaphragm (respiratory bias)
		components.ByName("organ:diaphragm").InputByName("autonomic_tone"),
	)
}

func wireHeart(components *component.Collection) error {
	if err := components.ByName("organ:heart").
		OutputByName("cardiac_activation").
		PipeTo(
			// Heart activity is observable
			components.ByName("physiology:observable_state").InputByName("heart_cardiac_activation"),
		); err != nil {
		return err
	}

	return components.ByName("organ:heart").
		OutputByName("rate").
		PipeTo(
			// Heart rate is observable
			components.ByName("physiology:observable_state").InputByName("heart_rate"),
		)
}

func wireDiaphragm(components *component.Collection) error {
	if err := components.ByName("organ:diaphragm").
		OutputByName("pleural_pressure").
		PipeTo(
			// Pleural pressure is observable
			components.ByName("physiology:observable_state").InputByName("pleural_pressure"),

			// And it drives the lungs
			components.ByName("organ:lung_left").InputByName("pleural_pressure"),
			components.ByName("organ:lung_right").InputByName("pleural_pressure"),
		); err != nil {
		return err
	}

	return components.ByName("organ:diaphragm").
		OutputByName("respiratory_rate").
		PipeTo(
			// Respiratory rate is observable
			components.ByName("physiology:observable_state").InputByName("respiratory_rate"),
		)
}

func wireRespiratoryBoundary(components *component.Collection) error {
	// Air flows from respiratory system to lungs and is observable
	return components.ByName("boundary:respiratory").OutputByName("inspired_gas").PipeTo(
		components.ByName("organ:lung_left").InputByName("inspired_gas"),
		components.ByName("organ:lung_right").InputByName("inspired_gas"),
		components.ByName("physiology:observable_state").InputByName("inspired_gas"),
	)
}

func wireLungs(components *component.Collection) error {
	for _, side := range []string{"left", "right"} {
		c := components.ByName("organ:lung_" + side)
		if err := c.OutputByName("volume").PipeTo(
			components.ByName("physiology:observable_state").InputByName("lung_" + side + "_volume"),
		); err != nil {
			return err
		}
		if err := c.OutputByName("flow").PipeTo(
			components.ByName("physiology:observable_state").InputByName("lung_" + side + "_flow"),
		); err != nil {
			return err
		}
		if err := c.OutputByName("alveolar_pressure").PipeTo(
			components.ByName("physiology:observable_state").InputByName("lung_" + side + "_alveolar_pressure"),
		); err != nil {
			return err
		}
		if err := c.OutputByName("exhaled_gas").PipeTo(
			components.ByName("physiology:observable_state").InputByName("lung_" + side + "_exhaled_gas"),
		); err != nil {
			return err
		}
	}
	return nil
}

// getComponents returns the collection of human components (organs, systems, etc.)
func getComponents() (*component.Collection, error) {
	resp, err := boundary.GetRespiratory()
	if err != nil {
		return nil, fmt.Errorf("boundary.GetRespiratory: %w", err)
	}
	autonomic, err := physiology.GetAutonomicCoordination()
	if err != nil {
		return nil, fmt.Errorf("physiology.GetAutonomicCoordination: %w", err)
	}
	obsState, err := physiology.GetObservableState()
	if err != nil {
		return nil, fmt.Errorf("physiology.GetObservableState: %w", err)
	}
	brain, err := organ.GetBrain()
	if err != nil {
		return nil, fmt.Errorf("organ.GetBrain: %w", err)
	}
	heart, err := organ.GetHeart()
	if err != nil {
		return nil, fmt.Errorf("organ.GetHeart: %w", err)
	}
	diaphragm, err := organ.GetDiaphragm()
	if err != nil {
		return nil, fmt.Errorf("organ.GetDiaphragm: %w", err)
	}
	lungLeft, err := organ.GetLung(common.Left)
	if err != nil {
		return nil, fmt.Errorf("organ.GetLung(left): %w", err)
	}
	lungRight, err := organ.GetLung(common.Right)
	if err != nil {
		return nil, fmt.Errorf("organ.GetLung(right): %w", err)
	}
	blood, err := da.GetBloodSystem()
	if err != nil {
		return nil, fmt.Errorf("da.GetBloodSystem: %w", err)
	}

	coll := component.NewCollection()
	if err := coll.Add(
		resp,
		blood,
		autonomic,
		obsState,
		brain,
		heart,
		diaphragm,
		lungLeft,
		lungRight,
	); err != nil {
		return nil, fmt.Errorf("failed to build human components: %w", err)
	}
	return coll, nil
}

func wireBloodSystem(components *component.Collection) error {
	lungLeft := components.ByName("organ:lung_" + string(common.Left))
	lungRight := components.ByName("organ:lung_" + string(common.Right))
	blood := components.ByName("da:blood_system")
	obsState := components.ByName("physiology:observable_state")

	// Alveolar gas from lungs to blood (left only — both are symmetric)
	if err := lungLeft.OutputByName("alveolar_gas").PipeTo(
		blood.InputByName("alveolar_gas"),
		obsState.InputByName("alveolar_gas"),
	); err != nil {
		return err
	}

	// Venous CO2 from blood to lungs
	if err := blood.OutputByName("venous_co2").PipeTo(
		lungLeft.InputByName("blood_co2"),
		lungRight.InputByName("blood_co2"),
		obsState.InputByName("venous_co2"),
	); err != nil {
		return err
	}

	return nil
}
