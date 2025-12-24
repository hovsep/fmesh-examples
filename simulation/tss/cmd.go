package tss

import "fmt"

type Command string

const (
	cmdPause  Command = "pause"
	cmdResume Command = "resume"
	cmdExit   Command = "exit"
	cmdHelp   Command = "help"
)

func showHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  exit - exit REPL")
	fmt.Println("  pause - pause simulation")
	fmt.Println("  resume - resume simulation")
	fmt.Println("  help - show this help")
}
