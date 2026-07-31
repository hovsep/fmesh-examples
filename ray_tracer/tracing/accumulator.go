package tracing

import (
	"context"
	"fmt"

	"github.com/hovsep/fmesh-examples/ray_tracer/common"
	"github.com/hovsep/fmesh-examples/ray_tracer/render"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// GetAccumulator returns the stateful component summing the contribution
// waves per tile. When a tile's last wave (bounce == MaxBounces) arrives,
// the finished samples are emitted downstream and the tile state is dropped
func GetAccumulator() *component.Component {
	return common.Must(component.New("accumulator",
		component.WithDescription("Sums contribution waves into per-sample colors"),
		component.WithInputs(common.PortIn),
		component.WithOutputs(common.PortOut),
		component.WithInitialState(func(state component.State) {
			state.Set("buffers", map[string][]render.Vec3{})
		}),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			buffers := this.State().Get("buffers").(map[string][]render.Vec3)

			return this.InputByName(common.PortIn).Signals().ForEach(func(sig *signal.Signal) error {
				key := fmt.Sprintf("%d/%d",
					common.ScalarInt(sig, common.ScalarFrame), common.ScalarInt(sig, common.ScalarYFrom))
				buffers[key] = render.Accumulate(buffers[key], sig.Payload().([]render.Contribution))

				if common.ScalarInt(sig, common.ScalarBounce) < render.MaxBounces {
					return nil
				}

				// Last wave of the tile: release the finished samples
				samples := buffers[key]
				delete(buffers, key)
				return this.OutputByName(common.PortOut).PutSignals(
					sig.MapPayload(func(any) any { return samples }).WithoutScalars(common.ScalarBounce))
			})
		}),
	))
}
