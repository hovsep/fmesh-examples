package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/internal"
	"github.com/hovsep/fmesh/component"
	"github.com/hovsep/fmesh/signal"
)

// A shell job as a mesh: every component is a real unix command, run as a
// separate process, and every pipe carries one command's stdout to the next
// command's stdin.
//
// A shell pipe `a | b | c` is a straight line. This job is not: one log is
// read once and analysed three ways at the same time, and the three results
// are stitched back into one report. In bash that needs `tee`, process
// substitution and temporary files; here it is just a few more pipes.
//
//	                ┌─▶ wc -l ─▶ awk ────────────────────────────────────────────────┐
//	read-stdin ─────┼─▶ cut ─▶ sort ─▶ uniq -c ─▶ sort -rn ─▶ head ─▶ awk ────────────┼─▶ report (cat)
//	   (cat)        └─▶ awk ─▶ cut ─▶ sort ─▶ uniq -c ─▶ sort -rn ─▶ head ─▶ awk ─────┘
//
// Run: go run . < access.log

const (
	portStart  = "start"
	portStdin  = "stdin"
	portStdout = "stdout"
)

// The report's inputs. A component reads its ports in name order, so these
// names are also the order of the sections in the report.
const (
	sectionRequests = "a_requests"
	sectionClients  = "b_clients"
	sectionErrors   = "c_errors"
)

func main() {
	fmt.Println("=== Unix Pipes ===")
	fmt.Println("Every component is a unix command; the mesh is the shell job.")
	fmt.Println()

	fm, err := getMesh(os.Stdin)
	if err != nil {
		fmt.Println("Failed to build mesh:", err)
		os.Exit(1)
	}

	handled, err := internal.HandleGraphFlag(fm)
	if err != nil {
		fmt.Println("Failed to generate graph:", err)
		os.Exit(1)
	}
	if handled {
		return
	}

	if isTerminal(os.Stdin) {
		fmt.Println("Type access log lines (ip method path status), then press Ctrl-D.")
		fmt.Println("Or feed the sample: go run . < access.log")
	}

	// Print which commands run in each cycle: commands in the same cycle are
	// separate processes running side by side.
	fm.SetupHooks(func(h *fmesh.Hooks) {
		h.AfterCycle(func(_ context.Context, cc *fmesh.CycleContext) error {
			if !cc.Cycle.HasActivatedComponents() {
				return nil
			}
			ran := make([]string, 0)
			for _, r := range cc.Cycle.ActivationResults().AllOrdered() {
				name := r.ComponentName()
				if component.IsWaitingForInput(r) {
					name += " (waiting)"
				}
				ran = append(ran, name)
			}
			fmt.Printf("  cycle %2d: %s\n", cc.Cycle.Number(), strings.Join(ran, ", "))
			return nil
		})
	})

	if err := fm.ComponentByName("read-stdin").InputByName(portStart).PutSignals(signal.New("go")); err != nil {
		fmt.Println("Failed to start the job:", err)
		os.Exit(1)
	}

	if _, err := fm.Run(context.Background()); err != nil {
		fmt.Println("Job failed:", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Print(report(fm))
}

// report returns what the last command of the job printed.
func report(fm *fmesh.FMesh) string {
	return fm.ComponentByName("report").OutputByName(portStdout).Signals().FirstPayloadOrDefault("")
}

// getMesh builds the job. The log is read once, from stdin, and every command
// after that gets it through pipes.
func getMesh(stdin io.Reader) (*fmesh.FMesh, error) {
	fm, err := fmesh.New("unix pipes",
		fmesh.WithDescription("an access-log report: unix commands as components, wired as a branching graph"),
		// A command that fails (bad flag, missing binary) fails the whole job.
		fmesh.WithErrorHandlingStrategy(fmesh.StopOnFirstErrorOrPanic),
	)
	if err != nil {
		return nil, err
	}

	source, err := newSource("read-stdin", stdin)
	if err != nil {
		return nil, err
	}
	reportCmd, err := newCommand("report", []string{"cat"}, sectionRequests, sectionClients, sectionErrors)
	if err != nil {
		return nil, err
	}
	if err := fm.AddComponents(source, reportCmd); err != nil {
		return nil, err
	}

	// Each branch is an ordinary shell pipeline: a | b | c.
	branches := []struct {
		section string
		argvs   [][]string
	}{
		{sectionRequests, [][]string{
			{"wc", "-l"},
			{"awk", `{ print "Requests: " $1 }`},
		}},
		{sectionClients, [][]string{
			{"cut", "-d", " ", "-f", "1"},
			{"sort"},
			{"uniq", "-c"},
			{"sort", "-rn"},
			{"head", "-n", "3"},
			{"awk", `BEGIN { print "Top clients:" } { printf "  %3d  %s\n", $1, $2 }`},
		}},
		{sectionErrors, [][]string{
			{"awk", `$4 >= 500`},
			{"cut", "-d", " ", "-f", "3"},
			{"sort"},
			{"uniq", "-c"},
			{"sort", "-rn"},
			{"head", "-n", "3"},
			{"awk", `BEGIN { print "Failing paths:" } { printf "  %3d  %s\n", $1, $2 }`},
		}},
	}

	for _, b := range branches {
		// Every branch starts from the same output: fmesh copies the signal
		// to each pipe, which is what `tee` does in a shell.
		from := source.OutputByName(portStdout)
		for i, argv := range b.argvs {
			cmd, err := newCommand(fmt.Sprintf("%s-%d-%s", b.section[2:], i+1, argv[0]), argv, portStdin)
			if err != nil {
				return nil, err
			}
			if err := fm.AddComponents(cmd); err != nil {
				return nil, err
			}
			if err := from.PipeTo(cmd.InputByName(portStdin)); err != nil {
				return nil, err
			}
			from = cmd.OutputByName(portStdout)
		}
		// The last command of each branch writes into its own report section.
		if err := from.PipeTo(reportCmd.InputByName(b.section)); err != nil {
			return nil, err
		}
	}

	return fm, nil
}

// newSource builds the head of the job: `cat` with the program's own stdin.
// It blocks until the input ends (Ctrl-D on a terminal, EOF on a file).
func newSource(name string, stdin io.Reader) (*component.Component, error) {
	return component.New(name,
		component.WithDescription("cat < stdin"),
		component.WithInputs(portStart),
		component.WithOutputs(portStdout),
		component.WithActivationFunc(func(ctx context.Context, this *component.Component) error {
			out, err := run(ctx, []string{"cat"}, stdin)
			if err != nil {
				return err
			}
			return this.OutputByName(portStdout).PutSignals(signal.New(out))
		}),
	)
}

// newCommand builds a component that runs argv as a process. It waits until
// every input has arrived, feeds them to the process's stdin in port-name
// order, and puts the whole stdout on its output as one signal.
func newCommand(name string, argv []string, inputs ...string) (*component.Component, error) {
	return component.New(name,
		component.WithDescription(commandLine(argv)),
		component.WithInputs(inputs...),
		component.WithOutputs(portStdout),
		component.WithActivationFunc(component.Sequential(
			// A join (the report) must not run on the first branch that
			// finishes: it keeps what arrived and waits for the rest.
			component.RequireInputs(inputs...),
			func(ctx context.Context, this *component.Component) error {
				var stdin strings.Builder
				for _, sig := range this.Inputs().Signals().All() {
					text, err := sig.As[string]()
					if err != nil {
						return err
					}
					stdin.WriteString(text)
				}

				out, err := run(ctx, argv, strings.NewReader(stdin.String()))
				if err != nil {
					return err
				}
				return this.OutputByName(portStdout).PutSignals(signal.New(out))
			},
		)),
	)
}

// run executes argv with the given stdin and returns its stdout. A non-zero
// exit is an error that carries what the command printed on stderr.
func run(ctx context.Context, argv []string, stdin io.Reader) (string, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Stdin = stdin
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("%s: %w: %s", commandLine(argv), err, strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("%s: %w", commandLine(argv), err)
	}
	return stdout.String(), nil
}

// commandLine renders argv the way you would type it into a shell.
func commandLine(argv []string) string {
	words := make([]string, len(argv))
	for i, arg := range argv {
		if arg == "" || strings.ContainsAny(arg, " \t\"'$<>|&;*?{}[]()") {
			arg = "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
		}
		words[i] = arg
	}
	return strings.Join(words, " ")
}

// isTerminal reports whether f is an interactive terminal rather than a file
// or a pipe, so the prompt is shown only to someone who can type.
func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
