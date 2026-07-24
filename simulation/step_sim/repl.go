package step_sim

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// REPL reads commands from stdin and forwards them to the simulation. It is the
// default CommandSource.
//
// Channel ownership: Run owns the channel it is given and is the only place that
// closes it, so Simulation.SendCommand must not be called once Run returns
// (sending on a closed channel panics). Callers sending from elsewhere must do
// so only while the REPL is still running.
type REPL struct{}

func NewREPL() *REPL { return &REPL{} }

// Run implements CommandSource: it reads stdin lines and forwards them on
// cmdChan, closing it on exit.
func (repl *REPL) Run(cmdChan chan Command) {
	defer close(cmdChan)

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

		// "exit" ends the session; everything else (help included, so custom
		// commands show up) goes to the simulation.
		if cmd == Exit {
			fmt.Println("Shutting down REPL...")
			return
		}
		cmdChan <- cmd
	}
}
