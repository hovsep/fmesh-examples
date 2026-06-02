package env

import (
	"fmt"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
)

const (
	meshName = "habitat_mesh"
)

// Habitat is a useful wrapper around F-Mesh that describes a habitat
type Habitat struct {
	FM *fmesh.FMesh
}

// NewHabitat builds the new habitat
func NewHabitat(factors *component.Collection) *Habitat {
	fm, err := fmesh.New(meshName,
		fmesh.WithUnlimitedCycles(),
		fmesh.WithTimeLimit(60*time.Second), // One mesh run (or 1 simulation tick) must not exceed this limit
	)
	if err != nil {
		panic(fmt.Sprintf("failed to create habitat mesh: %v", err))
	}

	habitat := &Habitat{FM: fm}
	return habitat.addFactors(factors)
}

// addFactors adds all exposure factors to the habitat mesh
func (h *Habitat) addFactors(factors *component.Collection) *Habitat {
	if !factors.AnyMatch(func(factor *component.Component) bool {
		return factor.Name() == "time"
	}) {
		panic("Time factor is required for the habitat mesh")
	}

	// Add all factors to the mesh
	if err := factors.ForEach(func(c *component.Component) error {
		return h.FM.AddComponents(c)
	}); err != nil {
		panic(fmt.Sprintf("failed to add factors to habitat mesh: %v", err))
	}

	// Connect inter-factor pipes
	if err := h.FM.Components().ForEach(func(c *component.Component) error {
		return h.connectToTimeFactor(c)
	}); err != nil {
		panic(fmt.Sprintf("failed to connect time factor: %v", err))
	}
	return h
}

// AddOrganisms adds organism components to the habitat mesh
func (h *Habitat) AddOrganisms(organisms ...*component.Component) *Habitat {
	for _, organism := range organisms {
		if err := h.FM.AddComponents(organism); err != nil {
			panic(fmt.Sprintf("failed to add organism to habitat: %v", err))
		}

		// Connect to habitat factors
		if err := h.FM.Components().ForEach(func(factor *component.Component) error {
			return factor.Outputs().ForEach(func(factorOutput *port.Port) error {
				// Check if the organism has relevant input
				orgInput := organism.Inputs().FindAny(func(p *port.Port) bool {
					return p.Name() == fmt.Sprintf("habitat_%s_%s", factor.Name(), factorOutput.Name())
				})

				if orgInput == nil {
					// No such input, skip
					return nil
				}

				return factorOutput.PipeTo(orgInput)
			})
		}); err != nil {
			panic(fmt.Sprintf("failed to connect organism to habitat factors: %v", err))
		}
	}
	return h
}

// getTimeFactor returns the time factor component
func (h *Habitat) getTimeFactor() *component.Component {
	return h.FM.Components().FindAny(func(c *component.Component) bool {
		return c.Name() == "time"
	})
}

// connectToTimeFactor connects the given component to the time factor
func (h *Habitat) connectToTimeFactor(c *component.Component) error {
	habitatTimeFactor := h.getTimeFactor()
	return c.Inputs().ForEach(func(p *port.Port) error {
		if p.Name() == "time" {
			return habitatTimeFactor.OutputByName("tick").PipeTo(p)
		}
		return nil
	})
}
