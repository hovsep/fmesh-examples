# Unix Pipes

A shell pipe `a | b | c` is a straight line. Real shell jobs rarely are: you read a log once, analyse it three ways, and stitch the answers back into one report. In bash that takes `tee`, process substitution and temporary files. In fmesh it is the same commands with a few more pipes, and the graph below is the job, drawn for you.

Every component here is a real unix command (`cat`, `wc`, `cut`, `sort`, `uniq`, `head`, `awk`) running as its own process. Every pipe carries one command's stdout to the next command's stdin.

```
                ┌─▶ wc -l ─▶ awk ──────────────────────────────────────────┐
read-stdin ─────┼─▶ cut ─▶ sort ─▶ uniq -c ─▶ sort -rn ─▶ head ─▶ awk ──────┼─▶ report (cat)
   (cat)        └─▶ awk ─▶ cut ─▶ sort ─▶ uniq -c ─▶ sort -rn ─▶ head ─▶ awk┘
```

The input is a web server access log (`ip method path status`). The report has three sections: how many requests there were, the top clients, and the paths that failed with a 5xx.

## How it works

- **A command is a component.** `newCommand(name, argv, inputs...)` runs `argv` with `os/exec`, feeds the signals on its inputs to the process's stdin, and puts the whole stdout on its `stdout` output as one signal. A non-zero exit is an activation error carrying the command's stderr, and `StopOnFirstErrorOrPanic` turns it into the job's failure.
- **The head of the job blocks on stdin.** `read-stdin` is `cat` reading the program's own stdin, so it waits for you to type (finish with Ctrl-D) or reads a file you redirect in.
- **Fan-out is free.** `read-stdin`'s output is piped into the first command of each branch; fmesh copies the signal to every pipe, which is exactly what `tee` does.
- **Concurrency is free.** Commands that are ready in the same cycle run at the same time, as separate processes. The run prints each cycle, so you can watch the three branches advance side by side.
- **The join waits.** The branches have different lengths, so the `report` would otherwise run on whichever section arrives first. `component.RequireInputs` keeps what arrived and waits for the rest; the run log shows `report (waiting)` until the longest branch finishes.
- **Port names order the report.** A component reads its inputs in port-name order, so naming the report's ports `a_requests`, `b_clients`, `c_errors` fixes the order of the sections, whatever order the branches finish in.
- Notable APIs: `component.Sequential` with `component.RequireInputs`, `this.Inputs().Signals()`, `sig.As[string]()`, a mesh-level `AfterCycle` hook reading `ActivationResults()` and `component.IsWaitingForInput`.

The same job in bash, for comparison:

```bash
cat access.log | tee \
  >(wc -l | awk '{ print "Requests: " $1 }' > a_requests) \
  >(cut -d ' ' -f 1 | sort | uniq -c | sort -rn | head -n 3 | awk '...' > b_clients) \
  | awk '$4 >= 500' | cut -d ' ' -f 3 | sort | uniq -c | sort -rn | head -n 3 | awk '...' > c_errors
wait; cat a_requests b_clients c_errors; rm a_requests b_clients c_errors
```

![Mesh graph](./unix%20pipes-graph.svg)

## Run

Needs the usual unix tools on `PATH` (Linux, macOS, or WSL).

```bash
# Feed the sample log
go run . < access.log

# Or type lines yourself and finish with Ctrl-D
go run .

# Regenerate the graph files (unix pipes-graph.dot / .svg)
FMESH_GRAPH=1 go run .
```
