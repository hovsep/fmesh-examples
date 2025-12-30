package step_sim

import (
	"bufio"
	"fmt"
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

func (r *REPL) Run() {
	fmt.Println("Starting REPL...")

	defer close(r.cmdChan)

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

		if r.handleCommand(cmd) {
			fmt.Println("Shutting down REPL...")
			return
		}
	}
}

// handleCommand processes a single REPL command and returns true if the REPL should be closed
func (r *REPL) handleCommand(cmd Command) bool {
	// Handle REPL-specific commands immediately and pass others to the channel
	switch cmd {
	case cmdExit:
		return true
	case cmdHelp:
		// Pass to simulation, so custom commands can be also displayed
		fallthrough
	default:
		r.cmdChan <- cmd
		return false
	}
}
