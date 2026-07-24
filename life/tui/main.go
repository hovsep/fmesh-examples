package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	defaultSocketPath = "/tmp/habitat_mesh.sock"
)

func main() {
	// Get socket path from args or use default
	socketPath := defaultSocketPath
	if len(os.Args) > 1 {
		socketPath = os.Args[1]
	}

	// Create model
	model, err := NewModel(socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing TUI: %v\n", err)
		os.Exit(1)
	}

	// Create Bubble Tea program
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),       // Use alternate screen buffer
		tea.WithMouseCellMotion(), // Enable mouse support
	)

	// Run the program
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
