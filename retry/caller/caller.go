package caller

import (
	"fmt"

	"github.com/hovsep/fmesh-examples/retry/service"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// Caller wraps an unstable service and handles request/response cycles
type Caller struct {
	component       *component.Component
	unstableService *service.UnstableService
}

// NewCaller creates a new caller component that wraps the unstable service
func NewCaller(name string, unstableService *service.UnstableService) *Caller {
	caller := &Caller{
		unstableService: unstableService,
	}

	caller.component = component.New(name).
		WithInputs("in_req").
		WithOutputs("out_success", "out_error").
		WithActivationFunc(func(this *component.Component) error {
			if len(this.InputByName("in_req").AllSignalsOrNil()) == 0 {
				return nil
			}
			req := this.InputByName("in_req").FirstSignalPayloadOrDefault("").(string)
			fmt.Printf("Proxy [%s]: Forwarding request '%s' to external service\n", this.Name(), req)

			resp, err := caller.unstableService.Call(req)
			if err != nil {
				fmt.Printf("Proxy [%s]: External service failed - %s\n", this.Name(), err)
				this.OutputByName("out_error").PutSignals(signal.New(req))
				return nil
			}
			fmt.Printf("Proxy [%s]: External service responded - %s\n", this.Name(), resp)
			this.OutputByName("out_success").PutSignals(signal.New(resp))
			return nil
		})

	return caller
}

// GetComponent returns the underlying F-Mesh component
func (c *Caller) GetComponent() *component.Component {
	return c.component
}
