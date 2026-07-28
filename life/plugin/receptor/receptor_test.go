package receptor_test

import (
	"testing"

	"github.com/hovsep/fmesh-examples/life/bloodstream"
	"github.com/hovsep/fmesh-examples/life/plugin/perfusion"
	"github.com/hovsep/fmesh-examples/life/plugin/receptor"
	"github.com/hovsep/fmesh/component"
)

// TestPluginsShareTheBloodPort is a regression test for a bug that only appeared
// on some runs.
//
// Both plugins need the component's blood input: perfusion to read what is being
// delivered, receptor to read what is being signalled. fmesh keeps plugins in a
// map, so which one initialises first is undefined -- and while both created the
// port unconditionally, an organ carrying both would fail to build on roughly
// half of all runs, with "port blood already exists".
//
// Building the same organ many times exercises both orders.
func TestPluginsShareTheBloodPort(t *testing.T) {
	for i := range 200 {
		c, err := component.New("organ:test",
			component.WithPlugins(
				perfusion.New(perfusion.Config{Organ: "test", O2PerMinute: 10}),
				receptor.For(bloodstream.HormoneAdrenaline),
			),
			component.WithInputs("time"),
		)
		if err != nil {
			t.Fatalf("build %d failed: %v", i, err)
		}
		if c.InputByName(bloodstream.SupplyPort) == nil {
			t.Fatalf("build %d has no blood input", i)
		}
		if c.OutputByName(bloodstream.ReturnPort) == nil {
			t.Fatalf("build %d has no blood output", i)
		}
	}
}

// TestReceptorAloneStillGetsItsPort covers the other order of the same problem:
// an organ that only listens to hormones, and is not perfused at all, still
// needs somewhere for the blood to arrive.
func TestReceptorAloneStillGetsItsPort(t *testing.T) {
	c, err := component.New("gland:test",
		component.WithPlugins(receptor.For(bloodstream.HormoneCortisol)),
	)
	if err != nil {
		t.Fatal(err)
	}
	if c.InputByName(bloodstream.SupplyPort) == nil {
		t.Fatal("a component with receptors but no perfusion should still receive blood")
	}
}

// TestUndeclaredHormonesAreNotFelt is the modelling claim the plugin makes: a
// tissue without the receptor is deaf to a hormone however much is circulating.
func TestUndeclaredHormonesAreNotFelt(t *testing.T) {
	c, err := component.New("organ:test",
		component.WithPlugins(receptor.For(bloodstream.HormoneAdrenaline)),
	)
	if err != nil {
		t.Fatal(err)
	}

	if got := receptor.Level(c, bloodstream.HormoneAdrenaline); got != 0 {
		t.Fatalf("a declared receptor should start at zero, got %v", got)
	}
	if got := receptor.Level(c, bloodstream.HormoneCortisol); got != 0 {
		t.Fatalf("an undeclared hormone should read zero, got %v", got)
	}
}
