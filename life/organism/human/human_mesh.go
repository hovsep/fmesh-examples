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
	"github.com/hovsep/fmesh-examples/life/organism/human/distributed_anatomy"
	"github.com/hovsep/fmesh-examples/life/organism/human/organ"
	"github.com/hovsep/fmesh-examples/life/organism/human/physiology"
	"github.com/hovsep/fmesh-examples/life/plugin/damage"
	"github.com/hovsep/fmesh-examples/life/plugin/perfusion"
	"github.com/hovsep/fmesh/component"
)

const (
	meshName = "human_mesh"
)

// getHumanMesh builds the mesh that simulates the human being
func getHumanMesh() (*fmesh.FMesh, error) {
	// Create the mesh
	mesh, err := fmesh.New(meshName,
		fmesh.WithUnlimitedCycles(),
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
	if err := wireCirculation(components); err != nil {
		return nil, fmt.Errorf("wireCirculation: %w", err)
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
	if err := wireMetabolism(components); err != nil {
		return nil, fmt.Errorf("wireMetabolism: %w", err)
	}
	if err := wireAffect(components); err != nil {
		return nil, fmt.Errorf("wireAffect: %w", err)
	}
	if err := wireDamage(components); err != nil {
		return nil, fmt.Errorf("wireDamage: %w", err)
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

// wireCirculation connects every perfused tissue to the bloodstream.
//
// The list is derived from the components themselves rather than written out:
// anything carrying the perfusion plugin gets the blood, and gives back what it
// has taken. Adding an organ to the circulation is therefore a matter of saying
// how much oxygen it needs, and nothing here changes -- the same principle the
// tick fan-out and the habitat's factor wiring already follow.
func wireCirculation(components *component.Collection) error {
	blood := components.ByName("da:blood_system")

	return components.ForEach(func(c *component.Component) error {
		if c == blood || !perfusion.IsPerfused(c) {
			return nil
		}

		// The organ reads what is being delivered...
		if err := blood.OutputByName("venous_blood").PipeTo(c.InputByName(perfusion.SupplyPort)); err != nil {
			return fmt.Errorf("supplying %s: %w", c.Name(), err)
		}
		// ...and returns what it has consumed and produced.
		if err := c.OutputByName(perfusion.ReturnPort).PipeTo(blood.InputByName("secretions")); err != nil {
			return fmt.Errorf("draining %s: %w", c.Name(), err)
		}
		return nil
	})
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
		obs := components.ByName("physiology:observable_state")
		if err := c.OutputByName("volume").PipeTo(
			obs.InputByName("lung_" + side + "_volume"),
		); err != nil {
			return err
		}
		if err := c.OutputByName("flow").PipeTo(
			obs.InputByName("lung_" + side + "_flow"),
		); err != nil {
			return err
		}
		if err := c.OutputByName("alveolar_pressure").PipeTo(
			obs.InputByName("lung_" + side + "_alveolar_pressure"),
		); err != nil {
			return err
		}
		if err := c.OutputByName("exhaled_gas").PipeTo(
			obs.InputByName("lung_" + side + "_exhaled_gas"),
		); err != nil {
			return err
		}
		if err := c.OutputByName("alveolar_gas").PipeTo(
			obs.InputByName("lung_" + side + "_alveolar_gas"),
		); err != nil {
			return err
		}
	}
	return nil
}

// wireMetabolism connects what the body takes in to what becomes of it.
//
// The chain is deliberately one-directional -- mouth to gut to reservoirs -- with
// the single exception of body_state, which physiology:physiological_state
// broadcasts before it integrates. That is what lets the kidney and the skin read
// the current hydration without forming a cycle the mesh could not converge.
func wireMetabolism(components *component.Collection) error {
	intake := components.ByName("controller:intake")
	ingestion := components.ByName("boundary:ingestion")
	gi := components.ByName("da:gi_tract")
	kidney := components.ByName("organ:kidney")
	skin := components.ByName("da:skin")
	bodyState := components.ByName("physiology:physiological_state")
	excretion := components.ByName("controller:excretion")
	physical := components.ByName("controller:physical_stress")
	obs := components.ByName("physiology:observable_state")

	// Mouth to stomach.
	if err := intake.OutputByName("intake_intent").PipeTo(
		ingestion.InputByName("intake_intent"),
	); err != nil {
		return err
	}
	if err := ingestion.OutputByName("nutrient_load").PipeTo(gi.InputByName("nutrient_load")); err != nil {
		return err
	}
	if err := ingestion.OutputByName("hydration_load").PipeTo(gi.InputByName("hydration_load")); err != nil {
		return err
	}

	// Stomach to body.
	if err := gi.OutputByName("absorption").PipeTo(bodyState.InputByName("absorption")); err != nil {
		return err
	}

	// Exertion sets the burn rate and tires the muscles.
	muscular := components.ByName("da:muscular_system")
	if err := physical.OutputByName("physical_load").PipeTo(
		bodyState.InputByName("physical_load"),
		muscular.InputByName("physical_load"),
	); err != nil {
		return err
	}

	// The blood carries the reservoir's glucose to the organs that draw on it
	// (the brain), so the gut ultimately reaches the brain through the bloodstream.
	if err := bodyState.OutputByName("glycemia").PipeTo(components.ByName("da:blood_system").InputByName("glucose")); err != nil {
		return err
	}

	// The reservoirs, broadcast to everything that needs to know them.
	if err := bodyState.OutputByName("body_state").PipeTo(
		kidney.InputByName("body_state"),
		skin.InputByName("body_state"),
		components.ByName("physiology:affect").InputByName("body_state"),
	); err != nil {
		return err
	}

	// Water leaving the body, from both routes.
	if err := kidney.OutputByName("losses").PipeTo(bodyState.InputByName("losses")); err != nil {
		return err
	}
	if err := skin.OutputByName("losses").PipeTo(bodyState.InputByName("losses")); err != nil {
		return err
	}

	// The skin's environmental heating/cooling reaches the core temperature.
	if err := skin.OutputByName("temperature_change").PipeTo(bodyState.InputByName("thermal")); err != nil {
		return err
	}

	// Voiding.
	if err := excretion.OutputByName("urine_out").PipeTo(kidney.InputByName("void")); err != nil {
		return err
	}
	if err := excretion.OutputByName("feces_out").PipeTo(gi.InputByName("void")); err != nil {
		return err
	}

	// Observation.
	return helper.MultiPipe(
		helper.PipeSpec{From: bodyState.OutputByName("hydration"), To: obs.InputByName("hydration")},
		helper.PipeSpec{From: bodyState.OutputByName("glycemia"), To: obs.InputByName("glycemia")},
		helper.PipeSpec{From: bodyState.OutputByName("energy"), To: obs.InputByName("energy")},
		helper.PipeSpec{From: bodyState.OutputByName("body_temperature"), To: obs.InputByName("body_temperature")},
		helper.PipeSpec{From: gi.OutputByName("stomach_fill"), To: obs.InputByName("stomach_fill")},
		helper.PipeSpec{From: skin.OutputByName("sweat_rate"), To: obs.InputByName("sweat_rate")},
		helper.PipeSpec{From: muscular.OutputByName("fatigue"), To: obs.InputByName("fatigue")},
	)
}

// wireAffect feeds the body's condition to the part of it that has opinions.
func wireAffect(components *component.Collection) error {
	affect := components.ByName("physiology:affect")
	gi := components.ByName("da:gi_tract")
	kidney := components.ByName("organ:kidney")
	obs := components.ByName("physiology:observable_state")

	// Bladder and bowel are both an observable number and something the body
	// feels, so each goes to two places.
	if err := kidney.OutputByName("bladder_fill").PipeTo(
		affect.InputByName("bladder_fill"),
		obs.InputByName("bladder_fill"),
	); err != nil {
		return err
	}
	if err := gi.OutputByName("bowel_fill").PipeTo(
		affect.InputByName("bowel_fill"),
		obs.InputByName("bowel_fill"),
	); err != nil {
		return err
	}

	if err := components.ByName("da:blood_system").OutputByName("venous_blood").PipeTo(
		affect.InputByName("venous_blood"),
	); err != nil {
		return err
	}
	if err := components.ByName("controller:physical_stress").OutputByName("physical_load").PipeTo(
		affect.InputByName("physical_load"),
	); err != nil {
		return err
	}
	if err := components.ByName("controller:mental_stress").OutputByName("mental_load").PipeTo(
		affect.InputByName("mental_load"),
	); err != nil {
		return err
	}

	return affect.OutputByName("feelings").PipeTo(obs.InputByName("feelings"))
}

// damagedOrgans maps each organ's short name (as the damage plugin and
// physiology:physiological_load know it) to the component that carries the plugin.
func damagedOrgans(components *component.Collection) map[string]*component.Component {
	return map[string]*component.Component{
		"brain":      components.ByName("organ:brain"),
		"heart":      components.ByName("organ:heart"),
		"diaphragm":  components.ByName("organ:diaphragm"),
		"lung_left":  components.ByName("organ:lung_left"),
		"lung_right": components.ByName("organ:lung_right"),
		"kidney":     components.ByName("organ:kidney"),
	}
}

// wireDamage connects the death cascade: what the body's condition does to its
// organs, what smoking does to the lungs, and how each organ's damage is observed.
func wireDamage(components *component.Collection) error {
	load := components.ByName("physiology:physiological_load")
	bodyState := components.ByName("physiology:physiological_state")
	blood := components.ByName("da:blood_system")
	obs := components.ByName("physiology:observable_state")
	organs := damagedOrgans(components)

	// The damage engine reads the reservoirs and the blood.
	if err := bodyState.OutputByName("body_state").PipeTo(load.InputByName("body_state")); err != nil {
		return err
	}
	if err := blood.OutputByName("venous_blood").PipeTo(load.InputByName("venous_blood")); err != nil {
		return err
	}

	for name, organ := range organs {
		// Reservoir stress injures the organ.
		if err := load.OutputByName(name + "_damage").PipeTo(organ.InputByName(damage.InputPort)); err != nil {
			return err
		}
		// Each organ's damage level is observable.
		if err := organ.OutputByName(damage.LevelOutput).PipeTo(obs.InputByName(name + "_damage")); err != nil {
			return err
		}
	}

	// Inhaled cigarette toxin damages both lungs directly.
	toxin := components.ByName("boundary:ingestion").OutputByName("substance_load")
	return toxin.PipeTo(
		organs["lung_left"].InputByName(damage.InputPort),
		organs["lung_right"].InputByName(damage.InputPort),
	)
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

	// Controllers are the body's command surface: every instruction from outside
	// the simulation ("eat", "run", "urinate") is addressed to one of these.
	intake, err := controller.GetIntake()
	if err != nil {
		return nil, fmt.Errorf("controller.GetIntake: %w", err)
	}
	excretion, err := controller.GetExcretion()
	if err != nil {
		return nil, fmt.Errorf("controller.GetExcretion: %w", err)
	}
	physical, err := controller.GetPhysical()
	if err != nil {
		return nil, fmt.Errorf("controller.GetPhysical: %w", err)
	}
	mental, err := controller.GetMental()
	if err != nil {
		return nil, fmt.Errorf("controller.GetMental: %w", err)
	}

	// The metabolic loop: what is swallowed, what becomes of it, what is lost,
	// and how the body feels about the result.
	ingestion, err := boundary.GetIngestion()
	if err != nil {
		return nil, fmt.Errorf("boundary.GetIngestion: %w", err)
	}
	gi, err := da.GetGITract()
	if err != nil {
		return nil, fmt.Errorf("da.GetGITract: %w", err)
	}
	kidney, err := organ.GetKidney()
	if err != nil {
		return nil, fmt.Errorf("organ.GetKidney: %w", err)
	}
	skin, err := da.GetSkin()
	if err != nil {
		return nil, fmt.Errorf("da.GetSkin: %w", err)
	}
	bodyState, err := physiology.GetPhysiologicalState()
	if err != nil {
		return nil, fmt.Errorf("physiology.GetPhysiologicalState: %w", err)
	}
	affect, err := physiology.GetAffect()
	if err != nil {
		return nil, fmt.Errorf("physiology.GetAffect: %w", err)
	}
	muscular, err := da.GetMuscularSystem()
	if err != nil {
		return nil, fmt.Errorf("da.GetMuscularSystem: %w", err)
	}
	load, err := physiology.GetPhysiologicalLoad()
	if err != nil {
		return nil, fmt.Errorf("physiology.GetPhysiologicalLoad: %w", err)
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
		intake,
		excretion,
		physical,
		mental,
		ingestion,
		gi,
		kidney,
		skin,
		bodyState,
		affect,
		muscular,
		load,
	); err != nil {
		return nil, fmt.Errorf("failed to build human components: %w", err)
	}
	return coll, nil
}

func wireBloodSystem(components *component.Collection) error {
	blood := components.ByName("da:blood_system")
	obsState := components.ByName("physiology:observable_state")

	for _, side := range []string{string(common.Left), string(common.Right)} {
		lung := components.ByName("organ:lung_" + side)

		// Lung airflow drives blood gas exchange (>0 inhaling, <0 exhaling)
		if err := lung.OutputByName("flow").PipeTo(
			blood.InputByName("airflow"),
		); err != nil {
			return err
		}

		// Venous blood feeds back to each lung
		if err := blood.OutputByName("venous_blood").PipeTo(
			lung.InputByName("venous_blood"),
		); err != nil {
			return err
		}
	}

	// The blood gases are observable, both as the composite the organs read and
	// as the individual readings a clinician would look at.
	return helper.MultiPipe(
		helper.PipeSpec{From: blood.OutputByName("venous_blood"), To: obsState.InputByName("venous_blood")},
		helper.PipeSpec{From: blood.OutputByName("spo2"), To: obsState.InputByName("blood_spo2")},
		helper.PipeSpec{From: blood.OutputByName("pao2"), To: obsState.InputByName("blood_pao2")},
		helper.PipeSpec{From: blood.OutputByName("paco2"), To: obsState.InputByName("blood_paco2")},
	)
}
