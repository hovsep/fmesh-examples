package step_sim

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

type REPL struct {
	cmdChan chan Command
}

func NewREPL(cmdChan chan Command) *REPL {
	return &REPL{
		cmdChan: cmdChan,
	}
}

func (repl *REPL) Run(r io.Reader) {
	fmt.Println("Starting REPL...")

	defer close(repl.cmdChan)

	scanner := bufio.NewScanner(r)
	for {
		_ = os.Stdout.Sync()
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				fmt.Println(fmt.Errorf("failed to read from stdIn: %w", err))
			}
			return
		}

		cmd := Command(strings.TrimSpace(scanner.Text()))

		if cmd == "" {
			continue
		}

		if repl.handleCommand(cmd) {
			fmt.Println("Shutting down REPL...")
			return
		}
	}
}

// handleCommand processes a single REPL command and returns true if the REPL should be closed
func (repl *REPL) handleCommand(cmd Command) bool {
	// Handle REPL-specific commands immediately and pass others to the channel
	switch cmd {
	case Exit:
		return true
	case Help:
		// Pass to simulation, so custom commands can be also displayed
		fallthrough
	default:
		repl.cmdChan <- cmd
		return false
	}
}
