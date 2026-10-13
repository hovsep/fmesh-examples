package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// A hypergraph is a graph whose edges join many nodes to many nodes. Here it
// is a chemical plant: substances are the nodes and every reaction is a
// hyperedge, joining all of its reactants to all of its products at once.
//
// The mesh is the hypergraph's incidence graph: one component per substance,
// one per reaction, one pipe per membership. A port in fmesh already joins
// many pipes, so a reaction needs exactly two: one input that every reactant
// pipes into, and one output that pipes to every product. What a plain edge
// cannot say, "only when all of these are here", is the reaction's activation
// function.
//
// Put raw materials on the shelf and the mesh works out everything the plant
// can make, one synthesis step per two cycles, every reaction whose inputs are
// ready firing in the same cycle.
//
// Run: go run .   (add -reactions file.txt to load another network)

const (
	portMade      = "made"      // substance: a reaction made it (or it came off the shelf)
	portPresent   = "present"   // substance: tell every reaction that uses it
	portReactants = "reactants" // reaction: one port, piped from every reactant
	portProducts  = "products"  // reaction: one port, piped to every product

	shelf = "shelf"
)

// Shelves are the raw materials a run starts from.
var (
	fullShelf = []string{"air", "H2O", "CH4", "NaCl", "CaCO3", "S", "SiO2"}
	noGas     = []string{"air", "H2O", "NaCl", "CaCO3", "S", "SiO2"}
)

func main() {
	reactions := flag.String("reactions", "", "reaction file to load instead of the bundled reactions.txt")
	flag.Parse()

	fmt.Println("=== Hypergraph ===")
	fmt.Println("A chemical plant where every reaction is a hyperedge: many reactants in, many products out.")
	fmt.Println()

	hg, err := LoadHypergraph(*reactions)
	if err != nil {
		fmt.Println("Bad reaction file:", err)
		os.Exit(1)
	}

	fm, err := getMesh(hg, fullShelf)
	if err != nil {
		fmt.Println("Failed to build mesh:", err)
		os.Exit(1)
	}
	handled, err := internal.HandleGraphFlag(fm)
	if err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}
	if handled {
		return
	}

	fmt.Printf("%d substances, %d reactions: %d components and %d pipes.\n",
		len(hg.Substances), len(hg.Reactions), fm.Components().Len(), hg.Incidences())
	big := hg.Reactions[0]
	for _, r := range hg.Reactions {
		if len(r.Reactants)+len(r.Products) > len(big.Reactants)+len(big.Products) {
			big = r
		}
	}
	fmt.Printf("The biggest hyperedge, %s, joins %d reactants to %d products through one input port and one output port.\n\n",
		big.Name, len(big.Reactants), len(big.Products))

	scenarios := []struct {
		title string
		shelf []string
		trace []string
	}{
		{"Everything on the shelf", fullShelf, []string{"NH3", "glass"}},
		{"The natural gas supply is cut", noGas, []string{"NH3"}},
	}
	for _, s := range scenarios {
		fm, err := getMesh(hg, s.shelf)
		if err != nil {
			fmt.Println("Failed to build mesh:", err)
			os.Exit(1)
		}
		res, err := synthesize(fm, hg, s.shelf)
		if err != nil {
			fmt.Println("Run failed:", err)
			os.Exit(1)
		}
		report(s.title, hg, res, s.trace)
	}
}

// Result is what a run found out.
type Result struct {
	Steps   [][]string          // reactions fired in each synthesis step
	Waiting map[int][]string    // per step: reactions still missing something
	MadeBy  map[string][]string // substance -> the reactions that made it first, or "shelf"
	Missing []string            // substances the plant cannot make
	Stuck   []string            // reactions that never fired, with what they lacked
}

// synthesize puts the shelf into the mesh, runs it and reads the outcome back
// from the cycles and from each component's state.
func synthesize(fm *fmesh.FMesh, hg *Hypergraph, raw []string) (*Result, error) {
	for _, f := range raw {
		c := fm.ComponentByName(f)
		if c == nil {
			return nil, fmt.Errorf("shelf: unknown substance %q", f)
		}
		if err := c.InputByName(portMade).PutSignals(signal.New(shelf)); err != nil {
			return nil, err
		}
	}
	info, err := fm.Run(context.Background())
	if err != nil {
		return nil, err
	}

	isReaction := map[string]bool{}
	for _, r := range hg.Reactions {
		isReaction[r.Name] = true
	}

	res := &Result{Waiting: map[int][]string{}, MadeBy: map[string][]string{}}
	for _, c := range info.Cycles.All() {
		var fired, waiting []string
		for _, r := range c.ActivationResults().AllOrdered() {
			if !isReaction[r.ComponentName()] {
				continue
			}
			if component.IsWaitingForInput(r) {
				waiting = append(waiting, r.ComponentName()+" ("+missingOf(r.ActivationError())+")")
				continue
			}
			fired = append(fired, r.ComponentName())
		}
		if len(fired)+len(waiting) == 0 {
			continue // a cycle of substances
		}
		res.Steps = append(res.Steps, inFileOrder(hg, fired))
		slices.Sort(waiting)
		res.Waiting[len(res.Steps)] = waiting
	}

	for _, f := range hg.Substances {
		by, err := fm.ComponentByName(f).State().GetTyped[[]string]("madeBy")
		if err != nil {
			res.Missing = append(res.Missing, f)
			continue
		}
		res.MadeBy[f] = by
	}
	for _, r := range hg.Reactions {
		c := fm.ComponentByName(r.Name)
		if fired, _ := c.State().GetTyped[bool]("fired"); fired {
			continue
		}
		have, _ := c.State().GetTyped[map[string]bool]("have")
		res.Stuck = append(res.Stuck, fmt.Sprintf("%s (no %s)", r.Name, strings.Join(lacking(r, have), ", ")))
	}
	return res, nil
}

// getMesh turns the hypergraph into a mesh: one component per substance and
// per reaction, and one pipe per incidence. A reaction's single input port is
// where all its reactants meet; its single output port fans out to all its
// products. That pair of ports is the hyperedge.
func getMesh(hg *Hypergraph, raw []string) (*fmesh.FMesh, error) {
	fm, err := fmesh.New("chemical plant",
		fmesh.WithDescription("a directed hypergraph: substances are nodes, reactions are hyperedges"),
	)
	if err != nil {
		return nil, err
	}

	for _, f := range hg.Substances {
		desc := hg.Names[f]
		if slices.Contains(raw, f) {
			desc += " · on the shelf"
		}
		sc, err := newSubstance(f, desc, hg)
		if err != nil {
			return nil, err
		}
		if err := fm.AddComponents(sc); err != nil {
			return nil, err
		}
	}
	for _, r := range hg.Reactions {
		rc, err := newReaction(r)
		if err != nil {
			return nil, err
		}
		if err := fm.AddComponents(rc); err != nil {
			return nil, err
		}
		for _, f := range r.Reactants {
			if err := fm.ComponentByName(f).OutputByName(portPresent).PipeTo(rc.InputByName(portReactants)); err != nil {
				return nil, err
			}
		}
		for _, f := range r.Products {
			if err := rc.OutputByName(portProducts).PipeTo(fm.ComponentByName(f).InputByName(portMade)); err != nil {
				return nil, err
			}
		}
	}
	return fm, nil
}

// newSubstance is a node. The first time something makes it, it remembers
// who and announces itself to every reaction that uses it. After that it
// stays quiet, which is also what stops the run: reaction networks have
// cycles (NO goes round through the nitric acid tower), but news does not.
func newSubstance(formula, name string, hg *Hypergraph) (*component.Component, error) {
	return component.New(formula,
		component.WithDescription(name),
		component.WithInputs(portMade),
		component.WithOutputs(portPresent),
		component.WithActivationFunc(func(ctx context.Context, this *component.Component) error {
			if this.State().Has("madeBy") {
				return nil // made again by another reaction: nothing new to tell
			}
			var by []string
			for _, p := range this.InputByName(portMade).Signals().AllPayloads() {
				by = append(by, p.(string))
			}
			this.State().Set("madeBy", inFileOrder(hg, by))
			return this.OutputByName(portPresent).PutSignals(signal.New(formula))
		}),
	)
}

// newReaction is a hyperedge. Its reactants arrive on one port, in whatever
// cycles they happen to be made; it keeps track of them in its state, and
// fires once, to all its products, when the last one is there.
func newReaction(r Reaction) (*component.Component, error) {
	return component.New(r.Name,
		component.WithDescription(r.String()),
		component.WithInputs(portReactants),
		component.WithOutputs(portProducts),
		component.WithInitialState(func(s component.State) {
			s.Set("have", map[string]bool{})
			s.Set("fired", false)
		}),
		component.WithActivationFunc(func(ctx context.Context, this *component.Component) error {
			have, err := this.State().GetTyped[map[string]bool]("have")
			if err != nil {
				return err
			}
			for _, p := range this.InputByName(portReactants).Signals().AllPayloads() {
				have[p.(string)] = true
			}
			if fired, _ := this.State().GetTyped[bool]("fired"); fired {
				return nil
			}
			if missing := lacking(r, have); len(missing) > 0 {
				// The arrived reactants are already in state, so this cycle's
				// signals can go.
				return fmt.Errorf("%w: needs %s", component.ErrWaitDroppingInputs, strings.Join(missing, ", "))
			}
			this.State().Set("fired", true)
			return this.OutputByName(portProducts).PutSignals(signal.New(r.Name))
		}),
	)
}

func lacking(r Reaction, have map[string]bool) []string {
	var missing []string
	for _, f := range r.Reactants {
		if !have[f] {
			missing = append(missing, f)
		}
	}
	return missing
}

// missingOf pulls the "needs ..." part back out of a waiting reaction's error.
func missingOf(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if i := strings.LastIndex(msg, "needs "); i >= 0 {
		return msg[i:]
	}
	return msg
}

// inFileOrder sorts reaction names the way the reaction file lists them, with
// the shelf first. Deterministic output, whatever order signals arrive in.
func inFileOrder(hg *Hypergraph, names []string) []string {
	rank := map[string]int{shelf: -1}
	for i, r := range hg.Reactions {
		rank[r.Name] = i
	}
	out := slices.Clone(names)
	slices.SortFunc(out, func(a, b string) int { return rank[a] - rank[b] })
	return slices.Compact(out)
}

func report(title string, hg *Hypergraph, res *Result, trace []string) {
	fmt.Println(title + ":")
	byName := map[string]Reaction{}
	for _, r := range hg.Reactions {
		byName[r.Name] = r
	}
	seen := map[string]bool{}
	for f, by := range res.MadeBy {
		if slices.Equal(by, []string{shelf}) {
			seen[f] = true
		}
	}
	for i, fired := range res.Steps {
		var made []string
		for _, name := range fired {
			for _, p := range byName[name].Products {
				if !seen[p] {
					seen[p] = true
					made = append(made, p)
				}
			}
		}
		line := fmt.Sprintf("  step %d: %d fired", i+1, len(fired))
		if len(made) > 0 {
			line += " → " + strings.Join(made, ", ")
		}
		fmt.Println(line)
		if w := res.Waiting[i+1]; len(w) > 0 {
			fmt.Printf("          waiting: %s\n", strings.Join(w, "; "))
		}
	}
	fmt.Printf("  → %d of %d substances in %d steps.", len(res.MadeBy), len(hg.Substances), len(res.Steps))
	if len(res.Missing) > 0 {
		fmt.Printf(" Cannot make: %s.", strings.Join(res.Missing, ", "))
	}
	fmt.Println()
	if len(res.Stuck) > 0 {
		fmt.Printf("  → never fired: %s\n", strings.Join(res.Stuck, "; "))
	}
	for _, f := range trace {
		what := f
		if hg.Names[f] != f {
			what += " (" + hg.Names[f] + ")"
		}
		fmt.Printf("\n  How the plant gets %s:\n", what)
		for _, l := range Trace(hg, res, f) {
			fmt.Println("    " + l)
		}
	}
	fmt.Println()
}

// Trace walks back from a substance through the first reaction that made it,
// down to the shelf. Substances already explained higher up are not repeated.
func Trace(hg *Hypergraph, res *Result, formula string) []string {
	byName := map[string]Reaction{}
	for _, r := range hg.Reactions {
		byName[r.Name] = r
	}
	var lines []string
	done := map[string]bool{}
	var walk func(f string, depth int)
	walk = func(f string, depth int) {
		pad := strings.Repeat("  ", depth)
		by := res.MadeBy[f]
		switch {
		case len(by) == 0:
			lines = append(lines, pad+f+" ✗ cannot be made")
			return
		case by[0] == shelf:
			lines = append(lines, pad+f+" from the shelf")
			return
		case done[f]:
			lines = append(lines, pad+f+" (see above)")
			return
		}
		done[f] = true
		line := pad + f + " ← " + by[0]
		if len(by) > 1 {
			line += " (also " + strings.Join(by[1:], ", ") + ")"
		}
		lines = append(lines, line)
		for _, in := range byName[by[0]].Reactants {
			walk(in, depth+1)
		}
	}
	walk(formula, 0)
	return lines
}
