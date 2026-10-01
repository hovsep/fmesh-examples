# Async Input

This example teaches how to drive a mesh from outside its own control loop: instead of feeding all input up front and calling `Run` once, an external goroutine injects signals on a ticker and calls `fm.Run` repeatedly, while another goroutine drains results from a Go channel as they arrive.

The scenario is an async HTTP crawler. A list of URLs is fed into the mesh one at a time, one every 3 seconds, each triggering an HTTP GET; the response headers (or errors) are collected and forwarded off the mesh into application code via a channel.

## How it works

- **`web crawler`** component — input `url`, outputs `errors` and `headers`. On activation it reads all queued URL signals and, for each, builds a request with `http.NewRequestWithContext` (so the run's context reaches the HTTP call) and sends it on a client with an 8-second timeout. It closes each response body, and puts either an error signal or a `map[string]http.Header` signal on the matching output.
- **`error logger`** component — input `error`. Piped from the crawler's `errors` output; prints any error it receives.
- The mesh (`"web scraper"`) is built with `fmesh.WithUnlimitedTime()` and `fmesh.WithUnlimitedCycles()` since crawling can be slow, and `fmesh.WithErrorHandlingStrategy(fmesh.StopOnFirstErrorOrPanic)`.
- Outside the mesh, a `time.Ticker` fires every 3 seconds; each tick, one URL is popped off the queue, pushed onto the crawler's `url` input via `InputByName("url").PutSignals(...)`, and `fm.Run(ctx)` is called synchronously for that single URL.
- After each run, if the `headers` output `HasSignals()`, its payloads are read with `Signals().AllPayloads()` and sent on `resultsChan`. The port needs no clearing: every `Run` starts by clearing all output ports.
- A second goroutine prints each result it reads from `resultsChan` (URL, header count, `Content-Type`) until the channel is closed (once the URL queue is empty), then signals `doneChan` so `main` can exit.
- Notable APIs: `component.WithActivationFunc`, `OutputByName(...).PipeTo(...)`, and driving `fm.Run` repeatedly from outside instead of once. Neither component checks for empty input: a component only activates when its inputs have signals.

![Mesh graph](./web%20scraper-graph.svg)

## Run

```bash
go run .
```

Set `FMESH_GRAPH=1` to regenerate the graph files (`web scraper-graph.dot` / `web scraper-graph.svg`) instead of running the crawl loop:

```bash
FMESH_GRAPH=1 go run .
```
