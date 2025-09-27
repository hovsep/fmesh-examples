package strategy

import (
	"fmt"

	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
)

// NewNoop returns a retry strategy that just forwards requests and responses without retrying
func NewNoop() *RetryStrategy {
	return NewRetryStrategy("noop", nil, func(this *component.Component) error {
		req_err := port.ForwardSignals(this.InputByName("req_in"), this.OutputByName("req_out"))
		if req_err != nil {
			return fmt.Errorf("failed to forward request: %w", req_err)
		}

		resp_err := port.ForwardSignals(this.InputByName("resp_in"), this.OutputByName("resp_out"))
		if resp_err != nil {
			return fmt.Errorf("failed to forward response: %w", resp_err)
		}
		return nil
	})
}
