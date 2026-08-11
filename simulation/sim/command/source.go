package command

import (
	"bufio"
	"context"
	"io"
	"os"
	"strings"
)

// Source feeds command lines into a running simulation.
//
// Run sends lines until its input is exhausted or ctx is cancelled, then
// returns. The two directions matter equally: a source that stops (end of a
// script, a user quitting) ends the session, and a session that stops (a
// scheduled exit, a finished batch) cancels ctx so the source stops too.
//
// The channel belongs to the session. A source only ever sends on it and must
// not close it.
type Source interface {
	Run(ctx context.Context, lines chan<- Line)
}

// Reader is a Source reading one command per line from any reader: a terminal,
// a piped script, a string in a test.
//
// Comments (#) and blank lines are skipped, so a script can be commented.
type Reader struct {
	in io.Reader
}

// NewReader returns a source reading from in.
func NewReader(in io.Reader) *Reader { return &Reader{in: in} }

// NewStdin returns a source reading from standard input. It is what a plain
// command-line simulation runs on.
func NewStdin() *Reader { return NewReader(os.Stdin) }

// Run implements Source.
//
// A read already in flight cannot be interrupted, so on cancellation this
// returns once the current line arrives (or the reader hits EOF) rather than
// instantly. Nothing waits on it: a session that has stopped does not wait for
// its source before returning.
func (r *Reader) Run(ctx context.Context, lines chan<- Line) {
	scanner := bufio.NewScanner(r.in)
	for scanner.Scan() {
		line := Line(strings.TrimSpace(scanner.Text()))
		if line == "" || strings.HasPrefix(string(line), "#") {
			continue
		}

		select {
		case lines <- line:
		case <-ctx.Done():
			return
		}
	}
}
