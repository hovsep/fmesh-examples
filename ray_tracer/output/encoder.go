package output

import (
	"fmt"
	"image"
	"image/gif"
	"os"
	"path/filepath"

	"github.com/hovsep/fmesh-examples/ray_tracer/common"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// GetEncoder returns the component accumulating paletted frames in its state
// and writing the animated GIF once the last frame arrives
func GetEncoder() *component.Component {
	return common.Must(component.New("encoder",
		component.WithDescription("Accumulates frames and encodes the animated GIF"),
		component.WithInputs(common.PortIn),
		component.WithInitialState(func(state component.State) {
			state.Set("animation", &gif.GIF{})
		}),
		component.WithActivationFunc(func(this *component.Component) error {
			anim := this.State().Get("animation").(*gif.GIF)

			return this.InputByName(common.PortIn).Signals().ForEach(func(sig *signal.Signal) error {
				anim.Image = append(anim.Image, sig.PayloadOrNil().(*image.Paletted))
				anim.Delay = append(anim.Delay, common.GifDelay)

				if common.ScalarInt(sig, common.ScalarFrame) < common.TotalFrames-1 {
					return nil
				}

				f, err := os.Create(common.OutputFile)
				if err != nil {
					return err
				}
				defer f.Close()

				if err := gif.EncodeAll(f, anim); err != nil {
					return err
				}
				absPath, _ := filepath.Abs(common.OutputFile)
				fmt.Println("Animation written to", absPath)
				return nil
			})
		}),
	))
}
