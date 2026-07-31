package tracing

import (
	"context"

	"github.com/hovsep/fmesh-examples/ray_tracer/common"
	"github.com/hovsep/fmesh-examples/ray_tracer/render"
	"github.com/hovsep/fmesh/component"
)

// GetIntersectors returns the collection of components finding scene
// intersections for a wave of rays. Each one receives both the primary wave
// (from its raygen) and the reflected waves (looped back from its shader)
func GetIntersectors() *component.Collection {
	intersectors := component.NewCollection()
	for i := range common.NumChains {
		common.MustOK(intersectors.Add(common.Must(component.New(common.WorkerName("intersector", i),
			component.WithDescription("Intersects a wave of rays with the geometry"),
			component.WithInputs(common.PortIn, common.PortGeometryIn),
			component.WithOutputs(common.PortOut),
			component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
				adoptGeometry(this)
				geometry := geometryFromState(this)

				return this.OutputByName(common.PortOut).PutSignalGroups(
					this.InputByName(common.PortIn).Signals().MapPayloads(func(p any) any {
						return geometry.IntersectAll(p.([]render.WeightedRay))
					}))
			}),
		))))
	}
	return intersectors
}
