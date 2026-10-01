package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const (
	componentFetch = "fetch"
	portName       = "name"
	portBody       = "body"
	stateAttempts  = "attempts"
	maxAttempts    = 5
	flakyFailures  = 2 // the fetcher fails this many times before it succeeds
)

var (
	errTemporary = errors.New("service temporarily unavailable")
	errNotFound  = errors.New("not found")
)

func main() {
	fmt.Println("=== Retry Demo ===")
	fmt.Println("A flaky fetcher fails twice with a temporary error, then succeeds.")
	fmt.Println("A permanent error (not found) is not retried.")
	fmt.Println()

	fm, err := getMesh()
	if err != nil {
		fmt.Println("Failed to build mesh:", err)
		os.Exit(1)
	}

	if err := internal.HandleGraphFlag(fm, true); err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}

	for _, name := range []string{"report.csv", "missing.csv"} {
		body, attempts, err := fetch(fm, name)
		if err != nil {
			fmt.Printf("%s: failed after %d attempt(s): %v\n", name, attempts, err)
			continue
		}
		fmt.Printf("%s: got %q after %d attempt(s)\n", name, body, attempts)
	}
}

// fetch runs the mesh once for one file name and reports how many attempts the activation took
func fetch(fm *fmesh.FMesh, name string) (string, int, error) {
	fetcher := fm.ComponentByName(componentFetch)
	fetcher.State().Set(stateAttempts, 0)
	if err := fetcher.InputByName(portName).PutSignals(signal.New(name)); err != nil {
		return "", 0, fmt.Errorf("put name: %w", err)
	}

	_, err := fm.Run(context.Background())
	attempts := fetcher.State().Get(stateAttempts).(int)
	if err != nil {
		return "", attempts, err
	}
	return fetcher.OutputByName(portBody).Signals().FirstPayloadOrDefault(""), attempts, nil
}

func getMesh() (*fmesh.FMesh, error) {
	fetcher, err := component.New(componentFetch,
		component.WithDescription("Fetches a file from a flaky service"),
		component.WithInputs(portName),
		component.WithOutputs(portBody),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			attempt := this.State().UpdateAndGet(stateAttempts, func(old any) any { return old.(int) + 1 }).(int)
			name := this.InputByName(portName).Signals().FirstPayloadOrDefault("")
			fmt.Printf("  attempt %d: fetching %s\n", attempt, name)

			if name == "missing.csv" {
				return fmt.Errorf("%s: %w", name, errNotFound)
			}
			if attempt <= flakyFailures {
				return fmt.Errorf("%s: %w", name, errTemporary)
			}
			return this.OutputByName(portBody).PutSignals(signal.New("contents of " + name))
		}),
		// WithRetry caps the attempts; WithRetryIf picks which errors are worth another one
		component.WithRetry(maxAttempts),
		component.WithRetryIf(func(ctx context.Context, attempt int, err error) bool {
			if !errors.Is(err, errTemporary) {
				return false // a permanent error will not go away by trying again
			}
			// Back off a little longer after each failure, but never past a canceled run
			select {
			case <-time.After(time.Duration(attempt) * 20 * time.Millisecond):
				return true
			case <-ctx.Done():
				return false
			}
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("fetch component: %w", err)
	}

	fm, err := fmesh.New("retry",
		fmesh.WithErrorHandlingStrategy(fmesh.StopOnFirstErrorOrPanic),
	)
	if err != nil {
		return nil, fmt.Errorf("new mesh: %w", err)
	}
	if err := fm.AddComponents(fetcher); err != nil {
		return nil, fmt.Errorf("add components: %w", err)
	}

	return fm, nil
}
