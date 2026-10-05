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
	"github.com/hovsep/fmesh/port"
	"github.com/hovsep/fmesh/signal"
)

// A rule engine whose rules live in a config file and whose engine is a mesh
// built from that file: one component per condition, one per rule, one per
// action. Edit rules.json and the next run has a different mesh; no code
// changes.
//
//	inbox ─┬─▶ if:<condition> ─┬─▶ rule:<name> (all / any) ─┬─▶ do:<action> ─▶ outbox
//	       └─▶ ...             └─▶ ...                      └─▶ ...
//
// A condition used by several rules is still one component, evaluated once
// per email and piped to every rule that needs it: the classic Rete-network
// trick, which the mesh gives for free.
//
// Run: go run .

const (
	portIn  = "in"
	portOut = "out"

	keyAction = "action" // on a signal leaving an action: which action it is
)

func main() {
	rulesPath := flag.String("rules", "", "rules file (default: the bundled rules.json)")
	inboxPath := flag.String("inbox", "", "emails to apply the rules to (default: the bundled inbox.json)")
	flag.Parse()

	fmt.Println("=== Rule Engine ===")
	fmt.Println("Rules come from a config file; the mesh is built from the rules.")
	fmt.Println()

	cfg, err := LoadConfig(*rulesPath)
	if err != nil {
		fmt.Println("Failed to load rules:", err)
		os.Exit(1)
	}

	fm, err := getMesh(cfg)
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

	source := *rulesPath
	if source == "" {
		source = "rules.json"
	}
	fmt.Printf("Built from %s: %d conditions, %d rules, %d actions — %d components in all.\n\n",
		source, len(cfg.Conditions), len(cfg.Rules), len(actionNames(cfg)), fm.Components().Len())

	emails, err := LoadInbox(*inboxPath)
	if err != nil {
		fmt.Println("Failed to load inbox:", err)
		os.Exit(1)
	}

	actions, err := apply(fm, emails)
	if err != nil {
		fmt.Println("Rule engine failed:", err)
		os.Exit(1)
	}

	for _, e := range emails {
		fmt.Printf("#%s  %-26s %-24q", e.ID, e.From, e.Subject)
		if len(actions[e.ID]) == 0 {
			fmt.Println("  · no rule matched")
			continue
		}
		fmt.Println("  →", strings.Join(actions[e.ID], ", "))
	}
}

// apply runs every email through the mesh at once and returns, per email id,
// the actions taken on it.
func apply(fm *fmesh.FMesh, emails []Email) (map[string][]string, error) {
	signals := make([]*signal.Signal, len(emails))
	for i, e := range emails {
		signals[i] = e.Signal()
	}
	if err := fm.ComponentByName("inbox").InputByName(portIn).PutSignals(signals...); err != nil {
		return nil, err
	}
	if _, err := fm.Run(context.Background()); err != nil {
		return nil, err
	}

	actions := make(map[string][]string)
	for _, sig := range fm.ComponentByName("outbox").OutputByName(portOut).Signals().All() {
		id := sig.PayloadOrDefault("")
		actions[id] = append(actions[id], sig.Meta().ValueOrDefault(keyAction, ""))
	}
	return actions, nil
}

// getMesh compiles the rules into a mesh.
func getMesh(cfg *Config) (*fmesh.FMesh, error) {
	fm, err := fmesh.New("rule engine",
		fmesh.WithDescription("rules from a config file compiled into a mesh: conditions → rules → actions"),
	)
	if err != nil {
		return nil, err
	}

	inbox, err := newRelay("inbox", "every email enters here")
	if err != nil {
		return nil, err
	}
	outbox, err := newRelay("outbox", "every action taken leaves here")
	if err != nil {
		return nil, err
	}
	if err := fm.AddComponents(inbox, outbox); err != nil {
		return nil, err
	}

	for _, c := range cfg.Conditions {
		cond, err := newCondition(c)
		if err != nil {
			return nil, err
		}
		if err := fm.AddComponents(cond); err != nil {
			return nil, err
		}
		if err := inbox.OutputByName(portOut).PipeTo(cond.InputByName(portIn)); err != nil {
			return nil, err
		}
	}

	for _, name := range actionNames(cfg) {
		action, err := newAction(name)
		if err != nil {
			return nil, err
		}
		if err := fm.AddComponents(action); err != nil {
			return nil, err
		}
		if err := action.OutputByName(portOut).PipeTo(outbox.InputByName(portIn)); err != nil {
			return nil, err
		}
	}

	for _, r := range cfg.Rules {
		rule, err := newRule(r)
		if err != nil {
			return nil, err
		}
		if err := fm.AddComponents(rule); err != nil {
			return nil, err
		}
		// One input port per condition, named after it.
		for _, c := range r.conditionNames() {
			if err := fm.ComponentByName("if:" + c).OutputByName(portOut).PipeTo(rule.InputByName(c)); err != nil {
				return nil, err
			}
		}
		for _, a := range r.Actions {
			if err := rule.OutputByName(portOut).PipeTo(fm.ComponentByName("do:" + a).InputByName(portIn)); err != nil {
				return nil, err
			}
		}
	}

	return fm, nil
}

// newRelay passes every signal through unchanged: the single way in and the
// single way out of the engine.
func newRelay(name, description string) (*component.Component, error) {
	return component.New(name,
		component.WithDescription(description),
		component.WithInputs(portIn),
		component.WithOutputs(portOut),
		component.WithActivationFunc(func(ctx context.Context, this *component.Component) error {
			return port.ForwardSignals(ctx, this.InputByName(portIn), this.OutputByName(portOut))
		}),
	)
}

// newCondition passes on only the emails that satisfy the condition.
func newCondition(c Condition) (*component.Component, error) {
	return component.New("if:"+c.Name,
		component.WithDescription(fmt.Sprintf("%s %s %v", c.Field, c.Op, c.Value)),
		component.WithInputs(portIn),
		component.WithOutputs(portOut),
		component.WithActivationFunc(func(ctx context.Context, this *component.Component) error {
			return port.ForwardWithFilter(ctx, this.InputByName(portIn), this.OutputByName(portOut), c.Matches)
		}),
	)
}

// newRule combines its conditions. Each condition arrives on its own port
// as the emails that passed it; "all" keeps the emails present on every
// port, "any" those present on at least one.
//
// Every condition sits one hop from the inbox, so all of a rule's inputs
// arrive in the same cycle: a port left empty means no email passed that
// condition, not that it is still on its way.
func newRule(r Rule) (*component.Component, error) {
	mode, conditions := "any", r.Any
	if len(r.All) > 0 {
		mode, conditions = "all", r.All
	}

	return component.New("rule:"+r.Name,
		component.WithDescription(fmt.Sprintf("%s of %s", mode, strings.Join(conditions, ", "))),
		component.WithInputs(conditions...),
		component.WithOutputs(portOut),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			passed := make(map[string]int) // email id → how many conditions it passed
			first := make(map[string]*signal.Signal)
			order := make([]string, 0)
			for _, name := range conditions {
				for _, sig := range this.InputByName(name).Signals().All() {
					id := sig.PayloadOrDefault("")
					if passed[id] == 0 {
						first[id] = sig
						order = append(order, id)
					}
					passed[id]++
				}
			}

			for _, id := range order {
				if mode == "any" || passed[id] == len(conditions) {
					if err := this.OutputByName(portOut).PutSignals(first[id]); err != nil {
						return err
					}
				}
			}
			return nil
		}),
	)
}

// newAction stamps every email it receives with its own name. Several rules
// may ask for the same action on the same email; it is taken once.
func newAction(name string) (*component.Component, error) {
	return component.New("do:"+name,
		component.WithDescription("take action: "+name),
		component.WithInputs(portIn),
		component.WithOutputs(portOut),
		component.WithActivationFunc(func(_ context.Context, this *component.Component) error {
			seen := make(map[string]bool)
			for _, sig := range this.InputByName(portIn).Signals().All() {
				id := sig.PayloadOrDefault("")
				if seen[id] {
					continue
				}
				seen[id] = true
				if err := this.OutputByName(portOut).PutSignals(signal.New(id).WithMeta(keyAction, name)); err != nil {
					return err
				}
			}
			return nil
		}),
	)
}

// actionNames lists every action the rules use, once each, sorted.
func actionNames(cfg *Config) []string {
	names := make([]string, 0)
	for _, r := range cfg.Rules {
		for _, a := range r.Actions {
			if !slices.Contains(names, a) {
				names = append(names, a)
			}
		}
	}
	slices.Sort(names)
	return names
}
