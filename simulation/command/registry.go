package command

import (
	"cmp"
	"fmt"
	"io"
	"slices"
	"sync"
)

// fallbackGroup labels commands registered without one.
const fallbackGroup = "Other"

// Registry is the set of commands a session accepts.
//
// It is safe for concurrent use: front ends read it (to complete a half-typed
// name) from their own goroutine while the simulation runs commands on its own.
type Registry struct {
	mu       sync.RWMutex
	commands map[string]Command
}

func NewRegistry() *Registry {
	return &Registry{commands: make(map[string]Command)}
}

// Add registers commands, replacing any with the same name.
func (r *Registry) Add(commands ...Command) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, c := range commands {
		r.commands[c.Name] = c
	}
}

// Lookup returns a command by name.
func (r *Registry) Lookup(name string) (Command, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	c, ok := r.commands[name]
	return c, ok
}

// Names returns every registered name, sorted. Front ends use it for completion
// and listings, so they never go out of step with what is actually accepted.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.commands))
	for name := range r.commands {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// WriteHelp lists the commands, grouped by section, with named sections in
// alphabetical order and the ungrouped ones last.
func (r *Registry) WriteHelp(out io.Writer) error {
	r.mu.RLock()
	groups := map[string][]Command{}
	nameWidth := 0
	for name, c := range r.commands {
		group := c.Group
		if group == "" {
			group = fallbackGroup
		}
		groups[group] = append(groups[group], c)
		nameWidth = max(nameWidth, len(name))
	}
	r.mu.RUnlock()

	order := make([]string, 0, len(groups))
	for group := range groups {
		if group != fallbackGroup {
			order = append(order, group)
		}
	}
	slices.Sort(order)
	if _, ok := groups[fallbackGroup]; ok {
		order = append(order, fallbackGroup)
	}

	if _, err := fmt.Fprintln(out, "Available commands:"); err != nil {
		return err
	}
	for _, group := range order {
		if _, err := fmt.Fprintf(out, "\n%s\n", group); err != nil {
			return err
		}

		commands := groups[group]
		slices.SortFunc(commands, func(a, b Command) int { return cmp.Compare(a.Name, b.Name) })
		for _, c := range commands {
			if _, err := fmt.Fprintf(out, "  %-*s  %s\n", nameWidth, c.Name, c.Description); err != nil {
				return err
			}
		}
	}
	return nil
}
