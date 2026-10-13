package main

import (
	"bufio"
	_ "embed"
	"errors"
	"fmt"
	"os"
	"strings"
)

//go:embed reactions.txt
var defaultReactions string

// Hypergraph is a directed hypergraph: substances are its nodes, reactions its
// hyperedges. Each reaction joins a set of reactants (its tail) to a set of
// products (its head).
type Hypergraph struct {
	Substances []string          // formulas, in file order
	Names      map[string]string // formula -> common name
	Reactions  []Reaction        // in file order
}

// Reaction is one hyperedge.
type Reaction struct {
	Name      string
	Reactants []string
	Products  []string
}

func (r Reaction) String() string {
	return strings.Join(r.Reactants, " + ") + " → " + strings.Join(r.Products, " + ")
}

// LoadHypergraph reads a reaction file, or the bundled reactions.txt when
// path is empty.
func LoadHypergraph(path string) (*Hypergraph, error) {
	text := defaultReactions
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		text = string(b)
	}
	return ParseHypergraph(text)
}

// ParseHypergraph reads "formula = name" and "reaction: A + B -> C + D" lines.
// Blank lines and lines starting with # are skipped.
func ParseHypergraph(text string) (*Hypergraph, error) {
	hg := &Hypergraph{Names: map[string]string{}}
	reactions := map[string]bool{}
	var errs []error

	sc := bufio.NewScanner(strings.NewReader(text))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if formula, name, ok := strings.Cut(line, "="); ok {
			formula, name = strings.TrimSpace(formula), strings.TrimSpace(name)
			if _, dup := hg.Names[formula]; dup {
				errs = append(errs, fmt.Errorf("line %d: substance %q is defined twice", n, formula))
				continue
			}
			hg.Names[formula] = name
			hg.Substances = append(hg.Substances, formula)
			continue
		}
		name, eq, ok := strings.Cut(line, ":")
		lhs, rhs, ok2 := strings.Cut(eq, "->")
		if !ok || !ok2 {
			errs = append(errs, fmt.Errorf("line %d: want \"formula = name\" or \"reaction: A + B -> C\", got %q", n, line))
			continue
		}
		r := Reaction{Name: strings.TrimSpace(name), Reactants: side(lhs), Products: side(rhs)}
		if reactions[r.Name] {
			errs = append(errs, fmt.Errorf("line %d: reaction %q is defined twice", n, r.Name))
		}
		if len(r.Reactants) == 0 || len(r.Products) == 0 {
			errs = append(errs, fmt.Errorf("line %d: reaction %q needs reactants and products", n, r.Name))
		}
		reactions[r.Name] = true
		hg.Reactions = append(hg.Reactions, r)
	}

	// Every formula a reaction uses must be a declared substance, and no
	// reaction may share its name with one: both become components.
	for _, r := range hg.Reactions {
		if _, clash := hg.Names[r.Name]; clash {
			errs = append(errs, fmt.Errorf("reaction %q has the name of a substance", r.Name))
		}
		for _, f := range append(append([]string{}, r.Reactants...), r.Products...) {
			if _, ok := hg.Names[f]; !ok {
				errs = append(errs, fmt.Errorf("reaction %q: unknown substance %q", r.Name, f))
			}
		}
	}
	return hg, errors.Join(errs...)
}

func side(s string) []string {
	var out []string
	for _, f := range strings.Split(s, "+") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

// Incidences is the number of (substance, reaction) memberships: one pipe each
// in the mesh.
func (hg *Hypergraph) Incidences() int {
	n := 0
	for _, r := range hg.Reactions {
		n += len(r.Reactants) + len(r.Products)
	}
	return n
}
