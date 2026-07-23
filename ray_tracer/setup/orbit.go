package setup

import (
	"github.com/hovsep/fmesh-examples/ray_tracer/common"
	"github.com/hovsep/fmesh-examples/ray_tracer/render"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// GetOrbit returns the component animating the camera: for every requested
// frame number it emits the viewpoint on the camera's orbit around the scene.
// It also ends the animation by emitting nothing once all frames are rendered
func GetOrbit() *component.Component {
	return common.Must(component.New("orbit",
		component.WithDescription("Moves the camera along its orbit, one step per frame"),
		component.WithInputs(common.PortNextFrame),
		component.WithOutputs(common.PortOut),
		component.WithActivationFunc(func(this *component.Component) error {
			return this.InputByName(common.PortNextFrame).Signals().ForEach(func(sig *signal.Signal) error {
				frame := sig.PayloadOrDefault(0).(int)
				if frame >= common.TotalFrames {
					// Animation finished: no output means no further activations,
					// so the mesh naturally comes to a halt
					return nil
				}

				viewpoint := render.OrbitPosition(frame, common.TotalFrames)
				return this.OutputByName(common.PortOut).PutSignals(
					signal.New(viewpoint).WithScalar(common.ScalarFrame, float64(frame)))
			})
		}),
	))
}
