package backoff

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/hovsep/fmesh/component"
)

func NewJittered() *component.Component {
	return component.New("jittered_backoff").
		WithInputs("in_req", "in_err").
		WithOutputs("out_req", "out_fail").
		WithActivationFunc(func(this *component.Component) error {
			if this.InputByName("in_err").IsEmpty() {
				return nil
			}

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
			capDuration, ok := this.Local["cap"].(time.Duration)
			if !ok {
				capDuration = 10 * time.Second
				this.Local["cap"] = capDuration
			}

			attempt++
			this.Local["attempt"] = attempt

			if attempt > maxRetries {
				fmt.Println("[jittered] retries exhausted")
				this.OutputByName("out_fail").PutSignals(this.InputByName("in_err").First())
				return nil
			}

			// exponential backoff with jitter
			exp := base * (1 << (attempt - 1))
			if exp > capDuration {
				exp = capDuration
			}
			// decorrelated jitter: random between base and exp
			sleep := base + time.Duration(rand.Int63n(int64(exp-base+1)))
			fmt.Printf("[jittered] attempt %d, sleeping %v\n", attempt, sleep)
			time.Sleep(sleep)

			// retry original request
			this.OutputByName("out_req").PutSignals(this.InputByName("in_req").First())
			return nil
		})

}
