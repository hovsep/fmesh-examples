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
	habitat := &Habitat{}
	habitat.FM = fmesh.NewWithConfig(meshName, &fmesh.Config{
		CyclesLimit: fmesh.UnlimitedCycles,
		TimeLimit:   10 * time.Second, // One mesh run (or 1 simulation tick) must not exceed this limit
	})

	habitat.addFactors(factors)

	return habitat
}

// addFactors adds all exposure factors to the habitat mesh
func (h *Habitat) addFactors(factors *component.Collection) {
	if !factors.AnyMatch(func(factor *component.Component) bool {
		return factor.Name() == "time"
	}) {
		panic("Time factor is required for the habitat mesh")
	}

	// Add all factors to the mesh
	factors.ForEach(func(c *component.Component) error {
		return h.FM.AddComponents(c).ChainableErr()
	}).ForEach(func(c *component.Component) error {
		return c.ChainableErr()
	})

	// Connect inter-factor pipes
	h.FM.Components().ForEach(func(c *component.Component) error {
		h.connectToTimeFactor(c)
		return h.FM.ChainableErr()
	})
}

func (h *Habitat) AddOrganisms(organisms ...*component.Component) {
	for _, organism := range organisms {
		h.FM.AddComponents(organism)

		// Connect to habitat factors
		h.FM.Components().ForEach(func(factor *component.Component) error {
			return factor.Outputs().ForEach(func(factorOutput *port.Port) error {
				// Check if the organism has relevant input
				orgInput := organism.Inputs().FindAny(func(p *port.Port) bool {
					return p.Name() == fmt.Sprintf("habitat_%s_%s", factor.Name(), factorOutput.Name())
				})

				if orgInput == nil {
					// No such input, skip
					return nil
				}

				return factorOutput.PipeTo(orgInput).ChainableErr()
			}).ChainableErr()
		})
	}
}

func (h *Habitat) getTimeFactor() *component.Component {
	return h.FM.Components().FindAny(func(c *component.Component) bool {
		return c.Name() == "time"
	})
}

func (h *Habitat) connectToTimeFactor(c *component.Component) {
	habitatTimeFactor := h.getTimeFactor()
	c.Inputs().ForEach(func(p *port.Port) error {
		if p.Name() == "time" {
			habitatTimeFactor.OutputByName("tick").PipeTo(p)
		}
		return p.ChainableErr()
	})
}
