// Package command is how a simulation is steered from outside: named actions,
// the lines that invoke them, and the sources those lines arrive from.
//
// It knows nothing about meshes or simulations. A command is a name, a
// description and a function; whatever it acts on it closes over.
package command

import (
	"io"
	"strings"
)

// Line is one raw command line, as typed, scripted or scheduled: a command name
// followed by its arguments.
type Line string

// Fields splits a line into the command name and its arguments. An empty line
// yields an empty name.
func (l Line) Fields() (name string, args []string) {
	fields := strings.Fields(string(l))
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], fields[1:]
}

// Handler runs a command.
//
// Anything the command wants to say goes to out; returning an error reports a
// problem — a usage mistake, a failure — and the caller decides how to show it.
// Handlers never print to stdout themselves, so they can be tested and their
// output can be routed anywhere (a terminal, a UI pane, a buffer).
type Handler func(out io.Writer, args []string) error

// Command is a named action.
type Command struct {
	Name        string
	Description string

	// Group is the help section this command appears under. Empty means the
	// fallback group, so only commands that want a named section set it.
	Group string

	// RawLine marks a command whose arguments are a whole line — a scenario, a
	// file path, another command. Callers that give lines a second reading
	// (splitting a scenario on separators, say) must leave these alone, or
	// defining something would run it instead.
	RawLine bool

	Run Handler
}
