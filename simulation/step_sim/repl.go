package step_sim

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// REPL reads commands from stdin and forwards them to the simulation.
//
// Channel ownership: the REPL is the owner of cmdChan and is the only place
// that closes it (see Run). Once Run returns, the channel is closed, so
// Simulation.SendCommand must not be called afterwards (sending on a closed
// channel panics). Callers that send commands from elsewhere are responsible
// for ensuring they do so only while the REPL is still running.
type REPL struct {
	cmdChan chan Command
}

func NewREPL(cmdChan chan Command) *REPL {
	return &REPL{
		cmdChan: cmdChan,
	}
}

func (repl *REPL) Run() {
	fmt.Println("Starting REPL...")

	defer close(repl.cmdChan)

	scanner := bufio.NewScanner(os.Stdin)
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
