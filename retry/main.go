package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/hovsep/fmesh-examples/retry/backoff"
	"github.com/hovsep/fmesh-examples/retry/caller"
	"github.com/hovsep/fmesh-examples/retry/controller"
	"github.com/hovsep/fmesh-examples/retry/service"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/signal"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

// This example demonstrates how to implement a retry proxy using F-Mesh components.
// The proxy sits between clients and external services, handling retry logic transparently.
// External services are decoupled from the mesh - F-Mesh only handles the retry orchestration.
// A Caller component acts as the proxy interface, a pluggable Backoff Strategy handles retry logic,
// and a Retry Controller manages final success/failure outcomes. Different backoff strategies can be used:
//
//	backoff.NewConstant(name, 200*time.Millisecond, 3)                    // Fixed delay
//	backoff.NewExponential(name, 100*time.Millisecond, 1000*time.Millisecond, 3) // Exponential delays
//	backoff.NewJittered(name, 100*time.Millisecond, 1000*time.Millisecond, 3)    // Random delays
func main() {
	// === EXTERNAL SERVICES (not part of F-Mesh) ===
	// These represent real external services that the retry proxy will call
	fmt.Println("=== Setting up External Services ===")

	// Service A: User service (90% failure rate to demonstrate retries)
	userService := service.NewUnstableService(0.9) // 90% failure rate
	fmt.Println("External Service A (User Service): 90% failure rate")

	// Service B: Payment service (90% failure rate to demonstrate retries)
	paymentService := service.NewUnstableService(0.9) // 90% failure rate
	fmt.Println("External Service B (Payment Service): 90% failure rate")

	// === F-MESH RETRY PROXY COMPONENTS ===
	// These components form the retry proxy that sits between clients and external services
	fmt.Println("\n=== Setting up F-Mesh Retry Proxy ===")

	// Proxy interface for Service A
	userServiceCaller := caller.NewCaller("user_service_proxy", userService)

	// Proxy interface for Service B
	paymentServiceCaller := caller.NewCaller("payment_service_proxy", paymentService)

	// Shared retry controller
	retryController := controller.NewRetryController("retry_controller")

	// Retry strategies (can be different per service)
	userBackoffStrategy := backoff.NewExponential("user_exponential_backoff", 100*time.Millisecond, 1000*time.Millisecond, 3)
	paymentBackoffStrategy := backoff.NewConstant("payment_constant_backoff", 200*time.Millisecond, 5) // Different strategy

	// Wire User Service Proxy
	userServiceCaller.GetComponent().OutputByName("out_error").PipeTo(userBackoffStrategy.InputByName("in_err"))
	userBackoffStrategy.OutputByName("out_req").PipeTo(userServiceCaller.GetComponent().InputByName("in_req"))
	userServiceCaller.GetComponent().OutputByName("out_success").PipeTo(retryController.GetComponent().InputByName("in_success"))
	userBackoffStrategy.OutputByName("out_fail").PipeTo(retryController.GetComponent().InputByName("in_fail"))

	// Wire Payment Service Proxy
	paymentServiceCaller.GetComponent().OutputByName("out_error").PipeTo(paymentBackoffStrategy.InputByName("in_err"))
	paymentBackoffStrategy.OutputByName("out_req").PipeTo(paymentServiceCaller.GetComponent().InputByName("in_req"))
	paymentServiceCaller.GetComponent().OutputByName("out_success").PipeTo(retryController.GetComponent().InputByName("in_success"))
	paymentBackoffStrategy.OutputByName("out_fail").PipeTo(retryController.GetComponent().InputByName("in_fail"))

	// Build the retry proxy mesh
	retryProxyMesh := fmesh.New("retry-proxy-mesh").
		WithComponents(
			userServiceCaller.GetComponent(),
			userBackoffStrategy,
			paymentServiceCaller.GetComponent(),
			paymentBackoffStrategy,
			retryController.GetComponent(),
		)

	// === CLIENT SIMULATION ===
	fmt.Println("\n=== Client Requests ===")

	// Client request 1: Get user data
	fmt.Println("Client Request 1: Getting user data...")
	userBackoffStrategy.InputByName("in_req").PutSignals(signal.New("user:123"))
	userServiceCaller.GetComponent().InputByName("in_req").PutSignals(signal.New("user:123"))

	// Client request 2: Process payment
	fmt.Println("Client Request 2: Processing payment...")
	paymentBackoffStrategy.InputByName("in_req").PutSignals(signal.New("payment:$100"))
	paymentServiceCaller.GetComponent().InputByName("in_req").PutSignals(signal.New("payment:$100"))

	// Run the retry proxy mesh (F-Mesh will automatically stop when no more signals to process)
	fmt.Println("\n=== Running Retry Proxy ===")
	runResult, err := retryProxyMesh.Run()
	if err != nil {
		fmt.Printf("Retry proxy finished with error: %s\n", err)
		return
	}

	fmt.Printf("\n=== Retry Proxy Results ===\n")
	fmt.Printf("Proxy processed requests in %d cycles over %v\n", runResult.Cycles.Len(), runResult.Duration)
	fmt.Println("Both services were called through the retry proxy, with different backoff strategies")
}
