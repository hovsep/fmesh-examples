package main

import (
	"fmt"
	"math/rand"
	"os"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

const (
	portIn  = "in"
	portOut = "out"
)

// This demo simulates a load balancing scenario using F-Mesh components.
// A central load balancer distributes incoming requests to multiple backend workers
// using a round-robin strategy. The system runs in waves (simulating traffic spikes),
// ensuring a fair distribution of a load even across multiple activation cycles.
// Each worker processes requests and returns a response, which the load balancer collects and emits.
// The demo showcases dynamic routing, stateful component behavior, and signal-based processing.
func main() {
	fm := getMesh()

	// Generate graphs if needed
	err := internal.HandleGraphFlag(fm)
	if err != nil {
		fmt.Println("Failed to generate graph: ", err)
		os.Exit(1)
	}

	// Run multiple waves (traffic spikes) to demonstrate that LB evenly distributes requests even when interrupted
	// you can play with the number of workers, waves and requests per wave to check that
	// #nosec G404 -- it's okay to use math/rand here for non-security purposes
	waves := 3 + rand.Intn(5)
	fmt.Println("Will run", waves, "waves")
	for i := 1; i <= waves; i++ {
		// #nosec G404 -- it's okay to use math/rand here for non-security purposes
		requestsPerWave := 5 + rand.Intn(10)
		fmt.Println("Wave", i, "will have", requestsPerWave, "requests")
		requests := signal.NewGroup()
		for j := 0; j < requestsPerWave; j++ {
			requests = requests.Add(signal.New(fmt.Sprintf("wave-%d req-%d", i, j)))
		}
		fm.ComponentByName("lb").
			InputByName(portIn).
			PutSignalGroups(requests)

		// Run
		_, err := fm.Run()
		if err != nil {
			fmt.Println("Load balancing finished with error:", err)
			os.Exit(1)
		}
	}

	fmt.Println("Load balancing finished successfully")

	// Extract results (responses)
	results := fm.ComponentByName("lb").OutputByName(portOut).Signals()
	if results.IsEmpty() {
		fmt.Println("No results found")
		os.Exit(2)
	}

	fmt.Println("Responses:")
	results.ForEach(func(sig *signal.Signal) error {
		fmt.Println(sig.PayloadOrDefault("").(string))
		return nil
	})
}

func getMesh() *fmesh.FMesh {
	workers := getWorkers("api-backend", 3)
	lb := getLoadBalancer("lb", workers)

	return fmesh.New("demo-load-balancing").
		AddComponents(lb).
		AddComponents(workers...)
}

func getWorkers(namePrefix string, number int) []*component.Component {
	workers := make([]*component.Component, number)
	for i := 0; i < number; i++ {
		worker := component.New(fmt.Sprintf("%s-%d", namePrefix, i)).
			AddInputs(portIn).
			AddOutputs(portOut).
			WithActivationFunc(func(this *component.Component) error {
				return this.InputByName(portIn).Signals().ForEach(func(sig *signal.Signal) error {
					// Receive request
					request := sig.PayloadOrDefault("").(string)

					// Process
					response := fmt.Sprintf("Request: %s processed by %s", request, this.Name())

					// Response
					return this.OutputByName(portOut).PutSignals(signal.New(response)).ChainableErr()
				}).ChainableErr()
			})
		workers[i] = worker
	}
	return workers
}

func getLoadBalancer(name string, workers []*component.Component) *component.Component {
	numWorkers := len(workers)

	if numWorkers < 1 {
		panic("at least 1 worker is required")
	}

	lb := component.New(name).
		WithDescription(fmt.Sprintf("Load balancer with %d workers", numWorkers)).
		AddInputs(portIn).                                // Ingress (requests to LB)
		AddIndexedInputs("upstream", 0, numWorkers-1).    // Upstream connections (responses from workers)
		AddIndexedOutputs("downstream", 0, numWorkers-1). // Downstream connections (requests to workers)
		AddOutputs(portOut).                              // Egress (responses from LB)
		WithInitialState(func(state component.State) {
			// We can go without it, as output ports will reflect the number of workers,
			// but it is more explicit and safe, as load balancer may have any other output ports
			// not connected to workers
			state.Set("workers_number", numWorkers)
		}).
		WithActivationFunc(func(this *component.Component) error {
			ingressPort := this.InputByName(portIn)
			egressPort := this.OutputByName(portOut)

			// Handle inbound traffic (ingress -> workers):
			lastWorkerIndex := this.State().GetOrDefault("last_worker_index", 0).(int)
			workersNum := this.State().Get("workers_number").(int)

			ingressPort.Signals().ForEach(func(sig *signal.Signal) error {
				// Round-robin distribution
				lastWorkerIndex %= workersNum

				this.OutputByName(indexedPortName("downstream", lastWorkerIndex)).PutSignals(sig)
				lastWorkerIndex++
				return nil
			})

			// Persist the last worker to continue evenly distribute signals even in the next activation cycles
			this.State().Set("last_worker_index", lastWorkerIndex)

			// Handle outbound traffic (workers -> egress)
			for i := 0; i < workersNum; i++ {
				// Just forward all signals from workers to egress
				err := port.ForwardSignals(this.InputByName(indexedPortName("upstream", i)), egressPort)
				if err != nil {
					return err
				}
			}

			return nil
		})

	// Connect workers to LB
	for i, w := range workers {
		lb.OutputByName(indexedPortName("downstream", i)).PipeTo(w.InputByName(portIn))
		w.OutputByName(portOut).PipeTo(lb.InputByName(indexedPortName("upstream", i)))
	}

	return lb
}

func indexedPortName(prefix string, index int) string {
	return fmt.Sprintf("%s%d", prefix, index)
}
