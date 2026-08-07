package simtest

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/simulation/command"
	"github.com/hovsep/fmesh-examples/simulation/session"
)

// Scenario is one command, the state it is issued in, and what it should do.
//
// It exists because "does this command work" is almost never a question about
// the value nearest the command. Raising the air temperature obviously raises
// the air temperature; the interesting claim is that the body notices, sweats,
// and loses water. A scenario states the whole chain at once, and one run of the
// simulation answers all of it.
type Scenario struct {
	// Name identifies this case. Several scenarios usually share a command --
	// a gentle effort and a punishing one are different claims.
	Name string

	// Command is the line under test.
	Command command.Line

	// Setup runs before anything is measured, for effects only visible against
	// something already in place: stopping requires having started.
	Setup []command.Line

	// Warm is how long to settle before the baseline is sampled; Baseline is how
	// long to sample it for. Settle is how long the effect is given after the
	// command, and Window is how much of the end of that counts as "after".
	Warm     time.Duration
	Baseline time.Duration
	Settle   time.Duration
	Window   time.Duration

	// Expect is the claim about numbers, keyed by observable name.
	Expect map[string]Check

	// ExpectSignals is the claim about traffic, keyed by signal source name.
	// It answers the questions a reading cannot: whether a port spoke at all,
	// what it carried, what it was labelled, and whether it fell silent.
	ExpectSignals map[string]SignalCheck

	// Backstop is applied to every observable Expect does not mention. It is
	// what makes the suite catch a mechanism nobody thought about, and it is the
	// reason a scenario is worth more than the assertions written in it.
	// Nil disables it.
	Backstop Check

	// Ignore lists observables to leave alone entirely -- neither expected nor
	// backstopped. Use it for values whose behaviour is genuinely undefined, and
	// say why in a comment, because everything in here is a hole in the net.
	Ignore []string
}

// Suite is the fixture a set of scenarios runs against.
type Suite struct {
	// NewSim builds a fresh simulation. Every scenario gets its own, so nothing
	// carries between them.
	NewSim func(t *testing.T) *session.Session

	// Observables is every number worth watching. Generate this from whatever
	// catalogue the model already has rather than writing it out, so that a
	// newly published value is watched without anybody remembering to add it.
	Observables []Observable

	// Sources is every port whose traffic is worth inspecting. Unlike
	// Observables there is no backstop over these: a mesh emits far too much to
	// assert about all of it, so a source is watched because a scenario asks.
	Sources []SignalSource

	// Tick is how much simulated time one sample covers. Only needed by checks
	// that talk about how long something took.
	Tick time.Duration

	// control caches the do-nothing run for each timing profile, since the same
	// profile is usually shared by many scenarios. The mutex is what lets the
	// scenarios themselves run in parallel: they share nothing else, since each
	// builds its own simulation.
	mu      sync.Mutex
	control map[string]map[string]Series
}

// Warm runs the control for every timing profile the scenarios use, before any
// of them start.
//
// Worth doing explicitly when the scenarios will run in parallel. Left to
// demand, several goroutines reach an uncached profile at once and each runs the
// same control simulation, which is the most expensive way to compute one
// answer. Once here, sequentially, is cheaper than n times at once.
func (s *Suite) Warm(t *testing.T, scenarios []Scenario) {
	t.Helper()
	for _, sc := range scenarios {
		s.controlFor(t, sc)
	}
}

// Run drives one scenario and judges everything it recorded.
func (s *Suite) Run(t *testing.T, sc Scenario) {
	t.Helper()

	if strings.Contains(string(sc.Command), ";") {
		// A line with a semicolon in it is routed as a multi-step scenario
		// rather than a command, so it would never reach the model and the test
		// would pass by measuring nothing.
		t.Fatalf("%s: a command may not contain ';'", sc.Name)
	}

	trace, marks := s.drive(t, sc.Setup, sc.Command, sc)
	control := s.controlFor(t, sc)

	ignored := make(map[string]bool, len(sc.Ignore))
	for _, name := range sc.Ignore {
		ignored[name] = true
	}

	// Sorted, so a failing run reports the same order every time.
	names := append([]string(nil), traceNames(s.Observables)...)
	sort.Strings(names)

	checked := 0
	for _, name := range names {
		if ignored[name] {
			continue
		}

		check, explicit := sc.Expect[name]
		if !explicit {
			if sc.Backstop == nil {
				continue
			}
			check = sc.Backstop
		}

		if trace.Published(name) == 0 {
			// Only an explicit expectation makes this an error. The backstop
			// sweeps every catalogued name, and a model is allowed to have
			// values it does not publish in every configuration.
			if explicit {
				t.Errorf("%s: %s was never published, so nothing could be asserted about it "+
					"(check the name, or whether the model publishes it at all)", sc.Name, name)
			}
			continue
		}

		observation := Observation{
			Name:      name,
			Before:    trace.Window(name, marks.baselineFrom, marks.baselineTo),
			After:     trace.Window(name, marks.windowFrom, marks.settleTo),
			During:    trace.Window(name, marks.baselineTo, marks.settleTo),
			Control:   control[name],
			PerSample: s.Tick,
		}

		checked++
		if err := check.Verify(observation); err != nil {
			label := "expected"
			if !explicit {
				label = "unexpected effect (backstop)"
			}
			t.Errorf("%s: %s: %s to %s -- %v",
				sc.Name, name, label, check.Describe(), err)
		}
	}

	for _, name := range sortedKeys(sc.ExpectSignals) {
		if !trace.Watched(name) {
			t.Errorf("%s: signal expectation names %q, which is not in the suite's Sources",
				sc.Name, name)
			continue
		}

		observation := SignalObservation{
			Name:   name,
			Before: trace.Traffic(name, marks.baselineFrom, marks.baselineTo),
			After:  trace.Traffic(name, marks.windowFrom, marks.settleTo),
		}

		checked++
		if err := sc.ExpectSignals[name].Verify(observation); err != nil {
			t.Errorf("%s: %s: expected to %s -- %v",
				sc.Name, name, sc.ExpectSignals[name].Describe(), err)
		}
	}

	if checked == 0 {
		t.Errorf("%s: nothing was checked; the observable list or the expectations are wrong", sc.Name)
	}

	// An expectation naming something not being watched is a typo that would
	// otherwise pass silently forever.
	watched := make(map[string]bool, len(s.Observables))
	for _, o := range s.Observables {
		watched[o.Name] = true
	}
	for name := range sc.Expect {
		if !watched[name] {
			t.Errorf("%s: expectation names %q, which is not being observed", sc.Name, name)
		}
	}
}

// marks are the sample indices bounding each phase of a run.
type marks struct {
	baselineFrom, baselineTo int
	windowFrom, settleTo     int
}

// drive runs one scenario and returns its trace and phase boundaries.
func (s *Suite) drive(t *testing.T, setup []command.Line, cmd command.Line, sc Scenario) (*Trace, marks) {
	t.Helper()

	sim := s.NewSim(t)
	trace := Watch(meshOf(sim), s.Observables, s.Sources)

	for _, line := range setup {
		sim.Do(line)
	}

	var m marks
	RunFor(sim, sc.Warm, func() { m.baselineFrom = trace.Mark() })
	RunFor(sim, sc.Baseline, func() { m.baselineTo = trace.Mark() })

	if cmd != "" {
		sim.Do(cmd)
	}

	window := sc.Window
	if window > sc.Settle {
		window = sc.Settle
	}
	RunFor(sim, sc.Settle-window, func() { m.windowFrom = trace.Mark() })
	RunFor(sim, window, func() { m.settleTo = trace.Mark() })

	return trace, m
}

// controlFor returns the same run with no command issued, so that a value which
// was drifting anyway is not credited to the command.
//
// Cached by timing profile, because scenarios share timings and the control is
// the same run every time.
func (s *Suite) controlFor(t *testing.T, sc Scenario) map[string]Series {
	t.Helper()

	key := fmt.Sprintf("%v|%v|%v|%v|%v", sc.Warm, sc.Baseline, sc.Settle, sc.Window, sc.Setup)

	s.mu.Lock()
	cached, ok := s.control[key]
	s.mu.Unlock()
	if ok {
		return cached
	}

	trace, m := s.drive(t, sc.Setup, "", sc)

	control := make(map[string]Series, len(s.Observables))
	for _, name := range traceNames(s.Observables) {
		control[name] = trace.Window(name, m.windowFrom, m.settleTo)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.control == nil {
		s.control = map[string]map[string]Series{}
	}
	s.control[key] = control
	return control
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func traceNames(observables []Observable) []string {
	names := make([]string, 0, len(observables))
	for _, o := range observables {
		names = append(names, o.Name)
	}
	return names
}

// meshOf reaches the mesh a session is driving.
func meshOf(sim *session.Session) *fmesh.FMesh {
	type mesher interface{ Mesh() *fmesh.FMesh }
	if m, ok := sim.Engine.(mesher); ok {
		return m.Mesh()
	}
	panic("simtest: this session is not driving an fmesh")
}
