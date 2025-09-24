package backoff

import (
	"fmt"
	"time"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

func NewConstant(name string, backoffDuration time.Duration, maxRetries int) *component.Component {
	return component.New(name).
		WithInputs("in_req", "in_err").
		WithOutputs("out_req", "out_fail").
		WithInitialState(func(state component.State) {
			state.Set("attempt", 0)
			state.Set("maxRetries", maxRetries)
			state.Set("backoff", backoffDuration)
		}).
		WithActivationFunc(func(this *component.Component) error {
			// Store original request if we have one
			if len(this.InputByName("in_req").AllSignalsOrNil()) > 0 {
				reqSignal := this.InputByName("in_req").AllSignalsOrNil()[0]
				this.State().Set("originalRequest", reqSignal)
			}

			if len(this.InputByName("in_err").AllSignalsOrNil()) == 0 {
				return nil
			}

			attempt := this.State().Get("attempt").(int)
			maxRetries := this.State().Get("maxRetries").(int)
			backoff := this.State().Get("backoff").(time.Duration)

			attempt++
			this.State().Set("attempt", attempt)

			if attempt > maxRetries {
				fmt.Printf("Backoff Strategy [%s]: Maximum attempts (%d) reached - giving up\n", this.Name(), maxRetries)
				errSignal := this.InputByName("in_err").AllSignalsOrNil()[0]
				this.OutputByName("out_fail").PutSignals(errSignal)
				return nil
			}

			fmt.Printf("Backoff Strategy [%s]: Attempt %d/%d - waiting %v before retry (constant backoff)\n", this.Name(), attempt, maxRetries, backoff)
			time.Sleep(backoff)

			// Retry with the stored original request
			if originalReq := this.State().Get("originalRequest"); originalReq != nil {
				this.OutputByName("out_req").PutSignals(originalReq.(*signal.Signal))
			}
			return nil
		})
}
