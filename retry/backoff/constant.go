package backoff

import (
	"fmt"
	"time"

	"github.com/hovsep/fmesh/component"
)

func NewConstatnt() *component.Component {
	return component.New("constant_backoff").
		WithInputs("in_req", "in_err").
		WithOutputs("out_req", "out_fail").
		WithActivationFunc(func(this *component.Component) error {
			if this.InputByName("in_err").IsEmpty() {
				return nil
			}

			attempt := this.Local["attempt"].(int)
			maxRetries := this.Local["maxRetries"].(int)
			backoff := this.Local["backoff"].(time.Duration)

			attempt++
			this.Local["attempt"] = attempt

			if attempt > maxRetries {
				this.OutputByName("out_fail").PutSignals(this.InputByName("in_err").First())
				return nil
			}

			fmt.Printf("[constant] attempt %d, backing off %v\n", attempt, backoff)
			time.Sleep(backoff)
			this.OutputByName("out_req").PutSignals(this.InputByName("in_req").First())
			return nil
		})

}
