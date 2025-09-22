package main

import (
	"errors"
	"fmt"
	"github.com/hovsep/fmesh-examples/retry/backoff"
	"math/rand"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// @TODO: fix everything
func main() {
	rand.Seed(time.Now().UnixNano())

	// --- Service B mock (unstable service) ---
	serviceB := func(req string) (string, error) {
		if rand.Float32() < 0.5 {
			return "", errors.New("service B failed")
		}
		return "response from B for: " + req, nil
	}

	// --- Caller Component (wraps service B) ---
	caller := component.New("caller").
		WithInputs("in_req").
		WithOutputs("out_success", "out_error").
		WithActivationFunc(func(this *component.Component) error {
			if this.InputByName("in_req").IsEmpty() {
				return nil
			}
			req := this.InputByName("in_req").FirstSignalPayloadOrDefault("").(string)
			fmt.Printf("Caller: calling service B with request: %s\n", req)

			resp, err := serviceB(req)
			if err != nil {
				fmt.Println("Caller: got error:", err)
				this.OutputByName("out_error").PutSignals(signal.New(req))
				return nil
			}
			fmt.Println("Caller: got success:", resp)
			this.OutputByName("out_success").PutSignals(signal.New(resp))
			return nil
		})

	// --- Retry Strategy: Exponential Backoff ---
	exponential := backoff.NewExponential(100*time.Millisecond, 1000*time.Millisecond, 3)

	// --- RetryController Component ---
	retryController := component.New("retry_controller").
		WithInputs("in_req", "in_success", "in_fail").
		WithOutputs("out_final").
		WithActivationFunc(func(this *component.Component) error {
			if !this.InputByName("in_success").IsEmpty() {
				success := this.InputByName("in_success").FirstSignalPayloadOrDefault("").(string)
				fmt.Println("RetryController: SUCCESS →", success)
				this.OutputByName("out_final").PutSignals(signal.New(success))
				return nil
			}
			if !this.InputByName("in_fail").IsEmpty() {
				fmt.Println("RetryController: FINAL FAILURE")
				this.OutputByName("out_final").PutSignals(signal.New("failure"))
				return nil
			}
			return nil
		})

	// --- Build mesh ---
	mesh := fmesh.New("retry-mesh").
		WithComponents(
			caller,
			exponential, // 🔄 can replace with constant_backoff component
			retryController,
		)

	// Wiring
	// A → RetryController
	// RetryController → Caller
	// Caller error → Strategy
	// Strategy retry → Caller
	// Strategy fail → RetryController
	// Caller success → RetryController
	mesh.ComponentByName("retry_controller").OutputByName("out_final") // sink, no wiring needed
	caller.InputByName("in_req").PipeFrom(exponential.OutputByName("out_req"))
	exponential.InputByName("in_req").PipeFrom(caller.InputByName("in_req"))
	exponential.InputByName("in_err").PipeFrom(caller.OutputByName("out_error"))
	retryController.InputByName("in_success").PipeFrom(caller.OutputByName("out_success"))
	retryController.InputByName("in_fail").PipeFrom(exponential.OutputByName("out_fail"))

	// --- Service A makes a request ---
	request := "hello from A"
	fmt.Println("Service A: sending request →", request)
	caller.InputByName("in_req").PutSignals(signal.New(request))

	// --- Run mesh cycles until finished ---
	for cycle := 1; cycle <= 20; cycle++ {
		fmt.Printf("\n=== Cycle %d ===\n", cycle)
		_, _ = mesh.Run()
		time.Sleep(200 * time.Millisecond)
	}
}
