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

func main() {
	fmt.Println("=== Load Balancer Simulation ===")
	fmt.Println("Architecture: A round-robin load balancer distributes incoming requests across N workers using indexed upstream/downstream ports.")
	fmt.Println("Indexed upstream/downstream ports connect the load balancer to each worker. Requests are assigned sequentially in rotation.")

	fm, err := getMesh()
	if err != nil {
		fmt.Println("Failed to build mesh:", err)
		os.Exit(1)
	}

	if err := internal.HandleGraphFlag(fm, true); err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}

	waves := 3 + rand.Intn(5)
	fmt.Println("Will run", waves, "waves")
	for i := 1; i <= waves; i++ {
		requestsPerWave := 5 + rand.Intn(10)
		fmt.Println("Wave", i, "will have", requestsPerWave, "requests")
		requests := signal.NewGroup()
		for j := range requestsPerWave {
			requests = requests.With(signal.New(fmt.Sprintf("wave-%d req-%d", i, j)))
		}
		fm.ComponentByName("lb").InputByName(portIn).PutSignalGroups(requests)

		_, err := fm.Run()
		if err != nil {
			fmt.Println("Load balancing finished with error:", err)
			os.Exit(1)
		}

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

	fmt.Println("=== Load Balancing Complete ===")
}

func getMesh() (*fmesh.FMesh, error) {
	workers, err := getWorkers("api-backend", 3)
	if err != nil {
		return nil, fmt.Errorf("workers: %w", err)
	}

	lb, err := getLoadBalancer("lb", workers)
	if err != nil {
		return nil, fmt.Errorf("load balancer: %w", err)
	}

	fm, err := fmesh.New("demo-load-balancing",
		fmesh.WithErrorHandlingStrategy(fmesh.StopOnFirstErrorOrPanic),
	)
	if err != nil {
		return nil, fmt.Errorf("new mesh: %w", err)
	}
	if err := fm.AddComponents(lb); err != nil {
		return nil, fmt.Errorf("add lb: %w", err)
	}
	if err := fm.AddComponents(workers...); err != nil {
		return nil, fmt.Errorf("add workers: %w", err)
	}
	return fm, nil
}

func getWorkers(namePrefix string, number int) ([]*component.Component, error) {
	workers := make([]*component.Component, number)
	for i := range number {
		worker, err := component.New(fmt.Sprintf("%s-%d", namePrefix, i),
			component.WithInputs(portIn),
			component.WithOutputs(portOut),
			component.WithActivationFunc(func(this *component.Component) error {
				return this.InputByName(portIn).Signals().ForEach(func(sig *signal.Signal) error {
					request := sig.PayloadOrDefault("").(string)
					response := fmt.Sprintf("Request: %s processed by %s", request, this.Name())
					return this.OutputByName(portOut).PutSignals(signal.New(response))
				})
			}),
		)
		if err != nil {
			return nil, fmt.Errorf("worker %d: %w", i, err)
		}
		workers[i] = worker
	}
	return workers, nil
}

func getLoadBalancer(name string, workers []*component.Component) (*component.Component, error) {
	numWorkers := len(workers)
	if numWorkers < 1 {
		return nil, fmt.Errorf("at least 1 worker is required")
	}

	lb, err := component.New(name,
		component.WithDescription(fmt.Sprintf("Load balancer with %d workers", numWorkers)),
		component.WithInputs(portIn),
		component.WithIndexedInputs("upstream", 0, numWorkers-1),
		component.WithIndexedOutputs("downstream", 0, numWorkers-1),
		component.WithOutputs(portOut),
		component.WithInitialState(func(state component.State) {
			state.Set("workers_number", numWorkers)
		}),
		component.WithActivationFunc(func(this *component.Component) error {
			ingressPort := this.InputByName(portIn)
			egressPort := this.OutputByName(portOut)

			lastWorkerIndex := this.State().GetOrDefault("last_worker_index", 0).(int)
			workersNum := this.State().Get("workers_number").(int)

			ingressPort.Signals().ForEach(func(sig *signal.Signal) error {
				lastWorkerIndex %= workersNum
				this.Logger().Printf("Routing %q -> worker-%d (%s)\n", sig.PayloadOrDefault(""), lastWorkerIndex, indexedPortName("downstream", lastWorkerIndex))
				this.OutputByName(indexedPortName("downstream", lastWorkerIndex)).PutSignals(sig)
				lastWorkerIndex++
				return nil
			})

			this.State().Set("last_worker_index", lastWorkerIndex)

			for i := range workersNum {
				if err := port.ForwardSignals(this.InputByName(indexedPortName("upstream", i)), egressPort); err != nil {
					return err
				}
			}

			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("new component: %w", err)
	}

	for i, w := range workers {
		if err := lb.OutputByName(indexedPortName("downstream", i)).PipeTo(w.InputByName(portIn)); err != nil {
			return nil, fmt.Errorf("pipe downstream %d: %w", i, err)
		}
		if err := w.OutputByName(portOut).PipeTo(lb.InputByName(indexedPortName("upstream", i))); err != nil {
			return nil, fmt.Errorf("pipe upstream %d: %w", i, err)
		}
	}

	return lb, nil
}

func indexedPortName(prefix string, index int) string {
	return fmt.Sprintf("%s%d", prefix, index)
}
