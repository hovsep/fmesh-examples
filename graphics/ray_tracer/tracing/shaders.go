package tracing

import (
	"context"

	"github.com/hovsep/fmesh-examples/graphics/ray_tracer/common"
	"github.com/hovsep/fmesh-examples/graphics/ray_tracer/render"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// GetShaders returns the collection of components converting hits into color
// contributions. Reflective hits spawn a secondary wave which is looped back
// into the chain's intersector.
//
// While bounce < render.MaxBounces the shader always emits the secondary wave,
// even an empty one, so every tile produces a deterministic number of waves
// and the accumulator knows when a tile is complete
func GetShaders() *component.Collection {
	shaders := component.NewCollection()
	for i := range common.NumChains {
		common.MustOK(shaders.Add(common.Must(component.New(common.WorkerName("shader", i),
			component.WithDescription("Shades hits and spawns the reflected wave"),
			component.WithInputs(common.PortIn, common.PortLightIn),
			component.WithOutputs(common.PortContribsOut, common.PortSecondaryOut),
			component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
				adoptLight(this)
				light := lightFromState(this)

				return this.InputByName(common.PortIn).Signals().ForEach(func(sig *signal.Signal) error {
					bounce := common.ScalarInt(sig, common.ScalarBounce)
					contribs, secondaries := render.Shade(light, sig.Payload().([]render.HitResult), bounce)

					err := this.OutputByName(common.PortContribsOut).PutSignals(
						sig.MapPayload(func(any) any { return contribs }))
					if err != nil {
						return err
					}

					if bounce >= render.MaxBounces {
						return nil
					}
					return this.OutputByName(common.PortSecondaryOut).PutSignals(
						sig.MapPayload(func(any) any { return secondaries }).
							WithScalar(common.ScalarBounce, float64(bounce+1)))
				})
			}),
		))))
	}
	return shaders
}
