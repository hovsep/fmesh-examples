package env

import (
	"fmt"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/life/common"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/plugin"
)

const (
	meshName = "habitat_mesh"
)

// Habitat is a useful wrapper around F-Mesh that describes a habitat
type Habitat struct {
	FM *fmesh.FMesh
}

// NewHabitat builds the new habitat
//
// Two naming conventions hold the habitat together, and both are declared here
// rather than carried out by hand further down. A component that declares an
// input called "time" gets the clock. A component that declares an input called
// "habitat_<factor>_<port>" gets that factor's output -- which is how an
// organism asks the world for exactly the parts of it that it cares about, and
// gets them whether it was added before those factors or after.
//
// Both used to be loops that walked the mesh looking for ports by name. The
// loops were correct and the failure mode was silent: anything added later that
// the loop no longer covered simply sat there, activating on inputs that never
// arrived, looking perfectly connected.
func NewHabitat(factors *component.Collection) (*Habitat, error) {
	fm, err := fmesh.New(meshName,
		// A single tick converges in a handful of cycles; this generous cap turns
		// an accidental non-converging run into a fast error instead of spinning
		// until the wall-clock time limit.
		fmesh.WithCyclesLimit(1000),
		fmesh.WithTimeLimit(60*time.Second), // One mesh run (or 1 simulation tick) must not exceed this limit
		fmesh.WithPlugins(
			plugin.AutowireBroadcastAs("tick", common.TimePort),
			plugin.AutowirePrefixed("habitat_"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create habitat mesh: %w", err)
	}

	habitat := &Habitat{FM: fm}
	return habitat.addFactors(factors)
}

// addFactors adds all exposure factors to the habitat mesh
func (h *Habitat) addFactors(factors *component.Collection) (*Habitat, error) {
	if !factors.AnyMatch(func(factor *component.Component) bool {
		return factor.Name() == "time"
	}) {
		return nil, fmt.Errorf("time factor is required for the habitat mesh")
	}

	if err := factors.ForEach(func(c *component.Component) error {
		return h.FM.AddComponents(c)
	}); err != nil {
		return nil, fmt.Errorf("failed to add factors to habitat mesh: %w", err)
	}
	return h, nil
}

// AddOrganisms adds organism components to the habitat mesh.
//
// Adding is all there is to it: the conventions declared in NewHabitat wire each
// organism to the world as it arrives.
func (h *Habitat) AddOrganisms(organisms ...*component.Component) (*Habitat, error) {
	for _, organism := range organisms {
		if err := h.FM.AddComponents(organism); err != nil {
			return nil, fmt.Errorf("failed to add organism to habitat: %w", err)
		}
	}
	return h, nil
}
