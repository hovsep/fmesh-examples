package tracing

import (
	"context"

	"github.com/hovsep/fmesh-examples/graphics/ray_tracer/common"
	"github.com/hovsep/fmesh-examples/graphics/ray_tracer/render"
	"github.com/hovsep/fmesh/component"
)

// GetRayGenerators returns the collection of components turning tiles into
// the primary wave of rays (bounce 0). One generator per chain
func GetRayGenerators() *component.Collection {
	generators := component.NewCollection()
	for i := range common.NumChains {
		common.MustOK(generators.Add(common.Must(component.New(common.WorkerName("raygen", i),
			component.WithDescription("Generates the primary rays for one tile"),
			component.WithInputs(common.PortIn),
			component.WithOutputs(common.PortOut),
			component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
				for sig := range this.InputByName(common.PortIn).Signals().All() {
					yFrom := common.ScalarInt(sig, common.ScalarYFrom)
					yTo := common.ScalarInt(sig, common.ScalarYTo)

					wave := sig.MapPayload(func(p any) any {
						return render.GenerateRays(p.(render.Camera), common.FrameWidth, common.FrameHeight, yFrom, yTo)
					}).WithMeta(common.ScalarBounce, 0.0)

					if err := this.OutputByName(common.PortOut).PutSignals(wave); err != nil {
						return err
					}
				}
				return nil
			}),
		))))
	}
	return generators
}
