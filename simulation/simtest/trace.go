package simtest

import (
	"context"
	"sync"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh/signal"
)

// Trace records what a set of observables did over a run.
//
// It samples once per mesh run, which for a step simulation is once per tick, by
// installing an AfterRun hook. That is the only lossless view of a running
// simulation: the telemetry sink is deliberately rate-limited so a fast run does
// not drown its front end, so a trace taken from there would miss most of what
// happened and silently report a smoother world than the real one.
type Trace struct {
	observables []Observable
	sources     []SignalSource

	mu      sync.Mutex
	series  map[string]Series
	present map[string]int
	// traffic holds the signals each watched port carried, one entry per sampled
	// cycle so a window can be sliced out of it the same way a Series can.
	// Signals are kept by pointer and treated as immutable once emitted.
	traffic map[string][][]*signal.Signal
	// last holds the most recent value of each observable, so a cycle in which
	// nothing was published carries the previous reading forward rather than
	// punching a hole in the series. A port publishes on its own schedule; a
	// gap means "no news", not "zero".
	last map[string]float64
	seen map[string]bool
}

// Watch installs a trace on a mesh and starts recording immediately.
//
// Hooks are additive, so this does not disturb any the caller has installed, and
// several traces can watch the same mesh.
func Watch(fm *fmesh.FMesh, observables []Observable, sources []SignalSource) *Trace {
	tr := &Trace{
		observables: observables,
		sources:     sources,
		series:      make(map[string]Series, len(observables)),
		present:     make(map[string]int, len(observables)),
		traffic:     make(map[string][][]*signal.Signal, len(sources)),
		last:        make(map[string]float64, len(observables)),
		seen:        make(map[string]bool, len(observables)),
	}

	fm.SetupHooks(func(hooks *fmesh.Hooks) {
		hooks.AfterRun(func(context.Context, *fmesh.FMesh) error {
			tr.sample(fm)

			// Never report a problem by returning an error here. An error from
			// AfterRun fails the mesh run, and the session answers a failed
			// advance by pausing itself and printing -- so a test would end
			// early and quietly pass rather than fail. Everything this collects
			// is judged after the run, where a failure is a failure.
			return nil
		})
	})
	return tr
}

func (t *Trace) sample(fm *fmesh.FMesh) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, o := range t.observables {
		value, ok := o.Read(fm)
		if ok {
			t.last[o.Name] = value
			t.seen[o.Name] = true
			t.present[o.Name]++
		} else if !t.seen[o.Name] {
			// Nothing has ever been published on this one, so there is not even
			// a previous reading to carry forward. Record nothing at all; the
			// runner reports a never-published observable as a broken name
			// rather than letting it read as a flat zero.
			continue
		}
		t.series[o.Name] = append(t.series[o.Name], t.last[o.Name])
	}

	// Traffic is recorded verbatim, including the empty cycles: a port that
	// carried nothing this cycle contributes an empty entry rather than no
	// entry, because "said nothing" is exactly what several checks are about.
	for _, s := range t.sources {
		t.traffic[s.Name] = append(t.traffic[s.Name], s.Read(fm))
	}
}

// Series returns everything recorded for one observable so far.
func (t *Trace) Series(name string) Series {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append(Series{}, t.series[name]...)
}

// Published reports how many times an observable was actually read, as opposed
// to carried forward. Zero means the name never resolved to anything.
func (t *Trace) Published(name string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.present[name]
}

// Mark returns the current length of the recording, so a caller can slice out
// the stretch between two moments.
func (t *Trace) Mark() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	longest := 0
	for _, s := range t.series {
		longest = max(longest, len(s))
	}
	return longest
}

// Window returns the readings an observable took between two marks. Either bound
// may be past the end, in which case it is clipped.
func (t *Trace) Window(name string, from, to int) Series {
	t.mu.Lock()
	defer t.mu.Unlock()

	s := t.series[name]
	from, to = min(max(from, 0), len(s)), min(max(to, 0), len(s))
	if from >= to {
		return nil
	}
	return append(Series{}, s[from:to]...)
}

// Names lists the observables this trace was asked to watch.
func (t *Trace) Names() []string {
	names := make([]string, 0, len(t.observables))
	for _, o := range t.observables {
		names = append(names, o.Name)
	}
	return names
}

// Traffic returns every signal a watched port carried between two marks,
// flattened across the sampled cycles, oldest first.
func (t *Trace) Traffic(name string, from, to int) []*signal.Signal {
	t.mu.Lock()
	defer t.mu.Unlock()

	cycles := t.traffic[name]
	from, to = min(max(from, 0), len(cycles)), min(max(to, 0), len(cycles))

	var all []*signal.Signal
	for _, cycle := range cycles[from:to] {
		all = append(all, cycle...)
	}
	return all
}

// Watched reports whether this trace was asked about a signal source at all.
func (t *Trace) Watched(name string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, ok := t.traffic[name]
	return ok
}
