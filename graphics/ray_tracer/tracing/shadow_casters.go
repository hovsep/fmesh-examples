package tracing

import (
	"context"

	"github.com/hovsep/fmesh-examples/graphics/ray_tracer/common"
	"github.com/hovsep/fmesh-examples/graphics/ray_tracer/render"
	"github.com/hovsep/fmesh/component"
)

// GetShadowCasters returns the collection of components testing which hits
// can actually see the light source
func GetShadowCasters() *component.Collection {
	casters := component.NewCollection()
	for i := range common.NumChains {
		common.MustOK(casters.Add(common.Must(component.New(common.WorkerName("shadow-caster", i),
			component.WithDescription("Casts shadow rays towards the light"),
			component.WithInputs(common.PortIn, common.PortGeometryIn, common.PortLightIn),
			component.WithOutputs(common.PortOut),
			component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
				adoptGeometry(this)
				adoptLight(this)
				geometry, light := geometryFromState(this), lightFromState(this)

				return this.OutputByName(common.PortOut).PutSignalGroups(
					this.InputByName(common.PortIn).Signals().MapPayloads(func(p any) any {
						return geometry.CastShadows(light, p.([]render.HitResult))
					}))
			}),
		))))
	}
	return casters
}
