package backoff

import (
	"fmt"
	"time"

	"github.com/hovsep/fmesh/component"
)

func NewExponential() *component.Component {
	return component.New("exponential_backoff").
		WithInputs("in_req", "in_err").
		WithOutputs("out_req", "out_fail").
		WithActivationFunc(func(this *component.Component) error {
			if this.InputByName("in_err").IsEmpty() {
				return nil
			}

			// initialize state if missing
			attempt, _ := this.Local["attempt"].(int)
			maxRetries, ok := this.Local["maxRetries"].(int)
			if !ok {
				maxRetries = 5
				this.Local["maxRetries"] = maxRetries
			}
			base, ok := this.Local["base"].(time.Duration)
			if !ok {
				base = time.Second
				this.Local["base"] = base
			}

			attempt++
			this.Local["attempt"] = attempt

			if attempt > maxRetries {
				fmt.Println("[exponential] retries exhausted")
				this.OutputByName("out_fail").PutSignals(this.InputByName("in_err").First())
				return nil
			}

			backoff := base * (1 << (attempt - 1))
			fmt.Printf("[exponential] attempt %d, sleeping %v\n", attempt, backoff)
			time.Sleep(backoff)

			// retry original request
			this.OutputByName("out_req").PutSignals(this.InputByName("in_req").First())
			return nil
		})

}
