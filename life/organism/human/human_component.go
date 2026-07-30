package human

import (
	"fmt"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh-examples/life/telemetry"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
)

// InnerMeshState is where the human component keeps the mesh that simulates its
// body. The body is a mesh wrapped as a single component, so from the outside
// there is otherwise no way to reach an organ -- which observation and tests need.
const InnerMeshState common.State = "inner_mesh"

// InnerMesh returns the mesh simulating the given human's body, or nil if the
// component is not a human.
func InnerMesh(c *component.Component) *fmesh.FMesh {
	if c == nil {
		return nil
	}
	mesh, _ := c.State().Get(InnerMeshState).(*fmesh.FMesh)
	return mesh
}

// Find returns the first human in a mesh, or nil if there is none.
//
// A human is recognised by what it is labelled rather than by its name, because
// the name belongs to the person -- there is a Leon in the habitat, not a
// "human" -- and anything looking for a body should not have to know who it is.
func Find(fm *fmesh.FMesh) *component.Component {
	return fm.Components().FindAny(func(c *component.Component) bool {
		return c.Labels().ValueIs("role", "organism") &&
			c.Labels().ValueIs("genus", "homo") &&
			c.Labels().ValueIs("species", "sapiens")
	})
}

// New returns a new human as a component (for simplicity we skip a clothing insulation factor, so the human being is naked)
func New(name string) (*component.Component, error) {
	mesh, err := getHumanMesh()
	if err != nil {
		return nil, fmt.Errorf("human.New: %w", err)
	}

	c, err := component.New("human-"+name,
		component.WithDescription("A human being"),
		component.WithLabel("role", "organism"),
		component.WithLabel("genus", "homo"),
		component.WithLabel("species", "sapiens"),
		component.WithInputs(
			"habitat_time_tick",
			"habitat_gas_environmental_gas",
			// Sunlight reaches the skin. Named to match the habitat's auto-wiring
			// convention habitat_<factor>_<output> (see env/habitat.go).
			"habitat_sun_uvi",
			// Commands from outside the simulation (eat, drink, exercise...).
			// Deliberately absent from validate(): commands are occasional, and
			// waiting for one would stop the body between them.
			ControlPort,
		),
		// Everything the body publishes, taken from the catalog so this list
		// cannot drift from what observable_state actually produces.
		component.WithOutputs(telemetry.Ports()...),
		component.WithActivationFunc(component.Sequential(
			validate(),
			sense(mesh),
			routeCommands(mesh),
			act(mesh),
			feedback(mesh),
		)),
		component.WithInitialState(func(state component.State) {
			state.Set(InnerMeshState, mesh)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create human component: %w", err)
	}
	return c, nil
}

// validate activation function
// Check if all required inputs are received
func validate() component.ActivationFunc {
	return func(this *component.Component) error {
		if !this.Inputs().ByNames("habitat_time_tick", "habitat_gas_environmental_gas").AllHaveSignals() {
			return component.ErrWaitingForInputsKeep
		}
		return nil
	}
}

// Sense activation function
// In this phase a human component receives inputs from the environment
// @TODO: so we have sense-act-feedback stages on human level. Let's think if we can use the same staged-approach on organ level. E.g. in sense we read stuff from blood and handle ctl signals, then in act we do something (diafragm contracts, lungs are ventilating) and in feedback - we just output our updated state
func sense(mesh *fmesh.FMesh) component.ActivationFunc {
	return func(this *component.Component) error {
		// Fan the tick out to every component in the body that keeps time.
		//
		// This used to be a hand-written list, which is a standing invitation to
		// add an organ and silently leave it frozen: it activates, sees no tick,
		// and quietly does nothing while looking perfectly wired. Deriving the
		// list from the ports themselves makes that impossible. The habitat wires
		// its own factors the same way (see env/habitat.go).
		tick := this.InputByName("habitat_time_tick")
		if err := mesh.Components().ForEach(func(c *component.Component) error {
			timePort := c.InputByName(common.TimePort)
			if timePort == nil {
				return nil
			}
			return port.ForwardSignals(tick, timePort)
		}); err != nil {
			return fmt.Errorf("failed to distribute time in human mesh: %w", err)
		}

		// Environmental air enters through the airway, and also reaches the skin,
		// which feels the ambient temperature carried on it.
		skin := mesh.ComponentByName("da:skin")
		if err := port.MultiForward(
			port.Pair{
				From: this.InputByName("habitat_gas_environmental_gas"),
				To:   mesh.ComponentByName("boundary:respiratory").InputByName("environmental_gas"),
			},
			port.Pair{
				From: this.InputByName("habitat_gas_environmental_gas"),
				To:   skin.InputByName("ambient_gas"),
			},
			// Sunlight falls on the skin.
			port.Pair{
				From: this.InputByName("habitat_sun_uvi"),
				To:   skin.InputByName("radiation"),
			},
		); err != nil {
			return fmt.Errorf("failed to forward environment into human mesh: %w", err)
		}
		return nil
	}
}

// Act activation function
// In this phase a human component runs inner mesh thus activating all organs and systems
//
// The body keeps being simulated after death: the brain has flatlined and
// is_alive is latched at 0, but the other organs wind down and their telemetry
// (including the damage that killed the body) keeps flowing to the UI. The mesh
// still converges because a failed organ that others depend on -- the diaphragm --
// holds a terminal output rather than going silent.
func act(mesh *fmesh.FMesh) component.ActivationFunc {
	return func(this *component.Component) error {
		_, err := mesh.Run()
		if err != nil {
			return fmt.Errorf("failed to run human mesh: %w", err)
		}
		return nil
	}
}

// Feedback activation function
// In this phase a human component propagates outputs from the inner mesh to the human component
func feedback(mesh *fmesh.FMesh) component.ActivationFunc {
	return func(this *component.Component) error {
		observableState := mesh.ComponentByName("physiology:observable_state")

		// Every published value is forwarded from the body's telemetry hub to
		// the matching output on the human component. Port names are identical
		// on both sides, so the catalog drives the whole hand-off.
		ports := telemetry.Ports()
		pairs := make([]port.Pair, 0, len(ports))
		for _, name := range ports {
			pairs = append(pairs, port.Pair{
				From: observableState.OutputByName(name),
				To:   this.OutputByName(name),
			})
		}

		if err := port.MultiForward(pairs...); err != nil {
			return fmt.Errorf("failed to forward signals from human mesh: %w", err)
		}
		return nil
	}
}
