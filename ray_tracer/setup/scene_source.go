// Package setup provides the components preparing each animation frame:
// the scene source, the orbiting camera and the director splitting frames
// into tiles for the parallel tracing chains.
package setup

import (
	"context"

	"github.com/hovsep/fmesh-examples/ray_tracer/common"
	"github.com/hovsep/fmesh-examples/ray_tracer/render"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

// GetSceneSource returns the mesh's single entry point: on kick-off it builds
// the scene and derives per-concern config signals from it — the geometry for
// the intersection tests and the light for shading (FBP-style initial
// information packets: each consumer receives only the data it needs, and no
// pointer is shared between components). It then forwards the kick-off to
// the orbit to start the animation
func GetSceneSource() *component.Component {
	return common.Must(component.New("scene",
		component.WithDescription("Builds the scene and emits its geometry and light"),
		component.WithInputs(common.PortIn),
		component.WithOutputs(common.PortGeometryOut, common.PortLightOut, common.PortStartOut),
		component.WithActivationFunc(func(ctx context.Context, this *component.Component) error {
			scene := render.NewScene()

			if err := this.OutputByName(common.PortGeometryOut).PutSignals(signal.New(scene.Geometry)); err != nil {
				return err
			}
			if err := this.OutputByName(common.PortLightOut).PutSignals(signal.New(scene.Light)); err != nil {
				return err
			}
			return port.ForwardSignals(ctx, this.InputByName(common.PortIn), this.OutputByName(common.PortStartOut))
		}),
	))
}
