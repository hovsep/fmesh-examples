package stepsim

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/simulation/sim/session"
	"github.com/hovsep/fmesh-examples/simulation/sim/sink"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

const step = 10 * time.Millisecond

// tickingMesh is the smallest thing that is still a time-step simulation: one
// component that activates once per run, counts the run, and writes a line to a
// "stream" port for telemetry to pick up.
//
// ticking says whether a hook feeds it a signal before each run; without one
// the mesh has nothing to do, which is how idleness is tested.
func tickingMesh(t *testing.T, ticking bool) (*fmesh.FMesh, *int) {
	t.Helper()

	runs := 0
	ticker, err := component.New("ticker",
		component.WithInputs("ctl"),
		component.WithOutputs("stream"),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			runs++
			return this.OutputByName("stream").PutPayloads(fmt.Sprintf("runs %d", runs))
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	fm, err := fmesh.New("test", fmesh.WithUnlimitedCycles(), fmesh.WithUnlimitedTime())
	if err != nil {
		t.Fatal(err)
	}
	if err := fm.AddComponents(ticker); err != nil {
		t.Fatal(err)
	}

	if ticking {
		fm.SetupHooks(func(hooks *fmesh.Hooks) {
			hooks.BeforeRun(func(_ context.Context, mesh *fmesh.FMesh) error {
				return mesh.ComponentByName("ticker").InputByName("ctl").PutSignals(signal.New("tick"))
			})
		})
	}
	return fm, &runs
}

func TestEngine_AdvanceRunsTheMeshAndMovesTheClock(t *testing.T) {
	fm, runs := tickingMesh(t, true)
	e := New(fm, step)

	for i := 1; i <= 3; i++ {
		result, err := e.Advance(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if result.Idle {
			t.Fatalf("advance %d reported idle, but the mesh ran", i)
		}
		if got, want := e.Now(), time.Duration(i)*step; got != want {
			t.Fatalf("after %d advances Now() = %v, want %v", i, got, want)
		}
	}

	if *runs != 3 {
		t.Fatalf("the mesh ran %d times, want 3", *runs)
	}
	if e.Steps() != 3 {
		t.Fatalf("Steps() = %d, want 3", e.Steps())
	}
}

func TestEngine_ReportsIdleWhenNothingActivates(t *testing.T) {
	// No tick hook, so after the first run there is nothing left to do. A
	// session with auto-pause stops on exactly this.
	fm, _ := tickingMesh(t, false)
	e := New(fm, step)

	result, err := e.Advance(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !result.Idle {
		t.Fatal("a mesh with nothing to do should report itself idle")
	}
	if result.Done {
		t.Fatal("idle is not done: input could still arrive")
	}
}

func TestEngine_WithClockDefersToTheMesh(t *testing.T) {
	fm, _ := tickingMesh(t, true)

	// A mesh that owns its own time: the engine must follow it rather than
	// counting steps in parallel.
	meshTime := 5 * time.Minute
	e := New(fm, step, WithClock(func() time.Duration { return meshTime }))

	if got := e.Now(); got != meshTime {
		t.Fatalf("Now() = %v, want the mesh's own time %v", got, meshTime)
	}
	if _, err := e.Advance(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := e.Now(); got != meshTime {
		t.Fatalf("Now() = %v; the engine should not have advanced a clock it does not own", got)
	}
}

func TestPortLines_ReadsWhatTheMeshPublished(t *testing.T) {
	fm, _ := tickingMesh(t, true)
	e := New(fm, step)

	lines, err := PortLines(fm, "ticker", "stream")
	if err != nil {
		t.Fatal(err)
	}
	if got := lines(); len(got) != 0 {
		t.Fatalf("before any run the port should be empty, got %v", got)
	}

	if _, err := e.Advance(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := lines(); len(got) != 1 || got[0] != "runs 1" {
		t.Fatalf("published %v, want [runs 1]", got)
	}

	// Each run replaces the last snapshot rather than accumulating.
	if _, err := e.Advance(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := lines(); len(got) != 1 || got[0] != "runs 2" {
		t.Fatalf("published %v, want [runs 2]", got)
	}
}

func TestPortLines_RejectsWhatIsNotThere(t *testing.T) {
	fm, _ := tickingMesh(t, true)

	// A typo must be a startup error, not a simulation that quietly publishes
	// nothing for the rest of its life.
	if _, err := PortLines(fm, "ticker", "nope"); err == nil {
		t.Fatal("expected an error for an unknown port")
	}
	if _, err := PortLines(fm, "nobody", "stream"); err == nil {
		t.Fatal("expected an error for an unknown component")
	}
}

// TestEngine_DrivenByASession is the end-to-end check that the seam fits: a
// real mesh, a real session, telemetry out to a real sink.
func TestEngine_DrivenByASession(t *testing.T) {
	fm, runs := tickingMesh(t, true)
	e := New(fm, step)

	lines, err := PortLines(fm, "ticker", "stream")
	if err != nil {
		t.Fatal(err)
	}

	target := sink.NewChannel(64)
	s := session.New(e, session.WithSink(target), session.WithTelemetry(lines))

	if err := s.RunFor(5 * step); err != nil {
		t.Fatal(err)
	}
	if err := target.Close(); err != nil {
		t.Fatal(err)
	}

	if *runs != 5 {
		t.Fatalf("the mesh ran %d times, want the 5 the deadline allowed", *runs)
	}

	var published []string
	for line := range target.Lines() {
		published = append(published, line)
	}
	if len(published) != 5 || published[4] != "runs 5" {
		t.Fatalf("published %v, want one line per step ending in 'runs 5'", published)
	}
}
