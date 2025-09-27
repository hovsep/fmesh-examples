package main

import (
	"fmt"
	"os"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/retry/strategy"
	"github.com/hovsep/fmesh/common"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	labelFrom = "from"
	labelTo   = "to"
)

func main() {
	// Prepare components
	serviceA := getService("A", []string{"B"})
	retryInterceptor := getRetryInterceptor(strategy.NewNoop())
	serviceB := getService("B", []string{"A"})

	// Build the mesh
	fm := fmesh.New("service mesh").
		WithDescription("simplified microservices architecture").
		WithComponents(serviceA, retryInterceptor, serviceB)

	// Piping
	fm.ComponentByName("service-A").OutputByName("to:B").PipeTo(fm.ComponentByName("service-B").InputByName("req"))
	// TODO fm.ComponentByName("service-B").OutputByName("to:A")

	// Set initial inputs
	req := signal.New("hello").WithLabels(common.LabelsCollection{
		labelFrom: "UI",
		labelTo:   "B",
	})

	fm.ComponentByName("service-A").InputByName("req").PutSignals(req)

	// Run the mesh
	runResult, err := fm.Run()
	if err != nil {
		fmt.Println("The mesh finished with error: ", err)
		os.Exit(1)
	}

	fmt.Printf("Mesh stopped after %d cycles and %s", runResult.Cycles.Len(), runResult.Duration)
}

func getService(name string, connectedServices []string) *component.Component {
	inputs := []string{"req"}
	outputs := []string{"resp"}

	for _, serviceName := range connectedServices {
		inputs = append(inputs, "from:"+serviceName)
		outputs = append(outputs, "to:"+serviceName)
	}

	return component.New("service-" + name).
		WithInputs(inputs...).
		WithOutputs(outputs...).
		WithActivationFunc(func(this *component.Component) error {
			// Get all input requests and route them to target services
			inputTraffic := this.InputByName("req").AllSignalsOrNil()
			for _, req := range inputTraffic {
				if !req.HasLabel(labelFrom) || !req.HasLabel(labelTo) {
					// Skip requests with incorrect routing keys
					continue
				}

				fromSvc := req.LabelOrDefault(labelFrom, "")
				toSvc := req.LabelOrDefault(labelTo, "")

				this.Logger().Printf("Forwarding a request - from: %s to: %s", fromSvc, toSvc)

				// Remove the target before routing
				req.DeleteLabels(labelTo)
				this.OutputByName("to:" + toSvc).PutSignals(req)
			}
			return nil
		})
}

func getRetryInterceptor(strategy *strategy.RetryStrategy) *component.Component {
	return component.New("retry").
		WithInitialState(strategy.StateInitializer).
		WithInputs("req_in", "resp_in").    // Request from caller, response from callee
		WithOutputs("req_out", "resp_out"). // Request to callee, response to caller
		WithActivationFunc(strategy.Logic)
}
