package imaging

import (
	"image"

	"github.com/hovsep/fmesh-examples/ray_tracer/common"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// GetAssembler returns the component merging tiles into a full frame.
// Thanks to the cycle barrier all tiles of a frame arrive in the same cycle
func GetAssembler() *component.Component {
	return common.Must(component.New("assembler",
		component.WithDescription("Merges tiles into a frame and requests the next one"),
		component.WithInputs(common.PortIn),
		component.WithOutputs(common.PortFrameOut, common.PortNextOut, common.PortProgressOut),
		component.WithActivationFunc(func(this *component.Component) error {
			img := image.NewRGBA(image.Rect(0, 0, common.FrameWidth, common.FrameHeight))
			frame := -1

			// Merge all the tiles which arrived in this cycle,
			// each tile knows its own place in the frame
			err := this.InputByName(common.PortIn).Signals().ForEach(func(sig *signal.Signal) error {
				frame = common.ScalarInt(sig, common.ScalarFrame)
				copy(img.Pix[common.ScalarInt(sig, common.ScalarYFrom)*img.Stride:], sig.PayloadOrNil().([]byte))
				return nil
			})
			if err != nil {
				return err
			}

			err = this.OutputByName(common.PortFrameOut).PutSignals(
				signal.New(img).WithScalar(common.ScalarFrame, float64(frame)))
			if err != nil {
				return err
			}
			err = this.OutputByName(common.PortProgressOut).PutSignals(
				signal.New(frame).WithScalar(common.ScalarFrame, float64(frame)))
			if err != nil {
				return err
			}

			// Feedback: ask the orbit for the next frame
			return this.OutputByName(common.PortNextOut).PutSignals(signal.New(frame + 1))
		}),
	))
}
