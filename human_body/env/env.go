package env

import (
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/human_body/env/factor"
	"github.com/hovsep/fmesh/component"
)

const (
	meshName = "env"
)

// GetMesh builds the environment mesh
func GetMesh() *fmesh.FMesh {
	mesh := fmesh.NewWithConfig(meshName, &fmesh.Config{
		CyclesLimit: 0,
		TimeLimit:   10 * time.Second, // One mesh run (or 1 simulation tick) must not exceed this limit
	})

	addFactors(mesh, getFactors())

	return mesh
}

// getFactors returns a collection of environment exposure factors
func getFactors() *component.Collection {
	factors := component.NewCollection().Add(
		factor.GetTimeComponent(),
		factor.GetSunComponent(),
		factor.GetTemperatureComponent(),
		factor.GetAirComponent(),
	)

	return factors
}

// addFactors adds all exposure factors to the mesh
func addFactors(envMesh *fmesh.FMesh, factors *component.Collection) {
	// Pick the time factor
	timeFactor := factors.FindAny(func(c *component.Component) bool {
		return c.Name() == "time"
	})

	// Connect the time factor to all other factors
	factors.Filter(func(c *component.Component) bool {
		return c.Name() != "time"
	}).ForEach(func(c *component.Component) error {
		return timeFactor.OutputByName("tick").PipeTo(c.InputByName("time")).ChainableErr()
	})

	// Add all factors to the mesh
	factors.ForEach(func(c *component.Component) error {
		return envMesh.AddComponents(c).ChainableErr()
	})
}

func AddOrganisms(envMesh *fmesh.FMesh, organisms ...*component.Component) {
	for _, organism := range organisms {
		envMesh.AddComponents(organism)
		envMesh.ComponentByName("time").OutputByName("tick").PipeTo(organism.InputByName("time"))
	}
}
