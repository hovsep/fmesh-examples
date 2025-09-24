package backoff

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/hovsep/fmesh/component"
)

func NewJittered(name string, baseDuration time.Duration, capDuration time.Duration, maxRetries int) *component.Component {
	return component.New(name).
		WithInputs("in_req", "in_err").
		WithOutputs("out_req", "out_fail").
		WithInitialState(func(state component.State) {
			state.Set("attempt", 0)
			state.Set("maxRetries", maxRetries)
			state.Set("base", baseDuration)
			state.Set("cap", capDuration)
		}).
		WithActivationFunc(func(this *component.Component) error {
			if len(this.InputByName("in_err").AllSignalsOrNil()) == 0 {
				return nil
			}

			attempt := this.State().Get("attempt").(int)
			maxRetries := this.State().Get("maxRetries").(int)
			base := this.State().Get("base").(time.Duration)
			capDuration := this.State().Get("cap").(time.Duration)

			attempt++
			this.State().Set("attempt", attempt)

			if attempt > maxRetries {
				fmt.Printf("[jittered] retries exhausted after %d attempts\n", attempt-1)
				errSignal := this.InputByName("in_err").AllSignalsOrNil()[0]
				this.OutputByName("out_fail").PutSignals(errSignal)
				return nil
			}

			// Exponential backoff with decorrelated jitter
			exp := base * (1 << (attempt - 1))
			if exp > capDuration {
				exp = capDuration
			}

			// Decorrelated jitter: random between base and exp
			jitterRange := int64(exp - base + 1)
			if jitterRange <= 0 {
				jitterRange = 1
			}
			sleep := base + time.Duration(rand.Int63n(jitterRange))

			fmt.Printf("[jittered] attempt %d, sleeping %v (range: %v - %v)\n", attempt, sleep, base, exp)
			time.Sleep(sleep)

			// Retry with the original request
			reqSignal := this.InputByName("in_req").AllSignalsOrNil()[0]
			this.OutputByName("out_req").PutSignals(reqSignal)
			return nil
		})
}
