# Retry

This example teaches how a component retries its own activation when it fails: `component.WithRetry` sets how many attempts an activation gets, and `component.WithRetryIf` decides after each failed attempt whether another one is worth making.

The scenario is a file fetcher talking to a flaky service. Fetching `report.csv` fails twice with a temporary error and succeeds on the third attempt, all inside one activation. Fetching `missing.csv` fails with a "not found" error, which no retry can fix, so it fails at once.

## How it works

- **`fetch`** — input `name`, output `body`. Each attempt bumps an `attempts` counter in the component's state, which `main` resets to 0 before each run. For `missing.csv` it returns `errNotFound`. Otherwise it returns `errTemporary` for the first two attempts and puts the file's contents on `body` on the third.
- **`WithRetry(5)`** — runs the activation function up to 5 times while it returns an error. Signals a failed attempt put on the outputs are removed before the next attempt, so retries do not pile up outputs.
- **`WithRetryIf(...)`** — called between attempts with the attempt number and its error. It returns `false` for anything that is not `errTemporary`, so the permanent error stops the retries after one attempt. For a temporary error it waits a little longer after each failure (20 ms, 40 ms, ...) and gives up if the run's context is done.
- When every allowed attempt fails, or `WithRetryIf` says stop, the activation fails with one error per attempt (`attempt 1 of 5: ...`), and `errors.Is` still finds the original error. With `StopOnFirstErrorOrPanic` that error is what `fm.Run` returns.

`main_test.go` runs both cases and checks the result and the number of attempts.

## Run

```bash
go run .

# Write the graph files (retry-graph.dot / .svg) instead of running
FMESH_GRAPH=1 go run .
```
