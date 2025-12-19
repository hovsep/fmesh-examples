package main

import (
	"bufio"
	"fmt"
	"os"
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
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		_ = os.Stdout.Sync()
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				fmt.Println(fmt.Errorf("reading standard input: %w", err))
			}
			return
		}

		cmd := Command(scanner.Text())

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
		showHelp()
		return false
	default:
		r.cmdChan <- cmd
		return false
	}
}
