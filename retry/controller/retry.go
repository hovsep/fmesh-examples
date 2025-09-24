package controller

import (
	"fmt"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// RetryController manages final success/failure outcomes from retry attempts
type RetryController struct {
	component *component.Component
}

// NewRetryController creates a new retry controller component
func NewRetryController(name string) *RetryController {
	controller := &RetryController{}

	controller.component = component.New(name).
		WithInputs("in_success", "in_fail").
		WithOutputs("out_final").
		WithActivationFunc(func(this *component.Component) error {
			// Check for success first
			if len(this.InputByName("in_success").AllSignalsOrNil()) > 0 {
				success := this.InputByName("in_success").FirstSignalPayloadOrDefault("").(string)
				fmt.Printf("Retry Controller: ✓ SUCCESS - Final response: %s\n", success)
				this.OutputByName("out_final").PutSignals(signal.New(success))
				return nil
			}
			// Check for final failure
			if len(this.InputByName("in_fail").AllSignalsOrNil()) > 0 {
				fmt.Println("Retry Controller: ✗ FAILED - All retry attempts exhausted")
				this.OutputByName("out_final").PutSignals(signal.New("failure"))
				return nil
			}
			return nil
		})

	return controller
}

// GetComponent returns the underlying F-Mesh component
func (r *RetryController) GetComponent() *component.Component {
	return r.component
}
