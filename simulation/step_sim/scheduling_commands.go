package step_sim

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/hovsep/fmesh"
)

// Scheduling and scripting command names.
const (
	Every        Command = "every"
	After        Command = "after"
	At           Command = "at"
	Jobs         Command = "jobs"
	Cancel       Command = "cancel"
	Script       Command = "script"
	Run          Command = "run"
	Scripts      Command = "scripts"
	ListPrograms Command = "programs"
	Stop         Command = "stop"
	Load         Command = "load"
)

// registerSchedulingCommands adds the commands that let a session express
// routines and scenarios rather than one instruction at a time.
func (s *Simulation) registerSchedulingCommands() {
	s.MeshCommands[Every] = NewMeshCommandDescriptorWithArgs(
		"run a command repeatedly in simulated time, e.g. 'every 1d excretion:defecate' (add 'x5' to limit the runs)",
		func(_ *fmesh.FMesh, args []string) { s.cmdEvery(args) })

	s.MeshCommands[After] = NewMeshCommandDescriptorWithArgs(
		"run a command once, after a stretch of simulated time, e.g. 'after 30m intake:water 250ml'",
		func(_ *fmesh.FMesh, args []string) { s.cmdAfter(args) })

	s.MeshCommands[At] = NewMeshCommandDescriptorWithArgs(
		"run a command once, at a simulated time since the run began, e.g. 'at 8h intake:food 500kcal'",
		func(_ *fmesh.FMesh, args []string) { s.cmdAt(args) })

	s.MeshCommands[Jobs] = NewMeshCommandDescriptor(
		"list scheduled commands", func(_ *fmesh.FMesh) { s.cmdJobs() })

	s.MeshCommands[Cancel] = NewMeshCommandDescriptorWithArgs(
		"cancel a scheduled command by id, or 'cancel all'",
		func(_ *fmesh.FMesh, args []string) { s.cmdCancel(args) })

	s.MeshCommands[Script] = NewMeshCommandDescriptorWithArgs(
		"name a scenario, e.g. 'script breakfast intake:food 400kcal; wait 30m; activity:start 3 15m'",
		func(_ *fmesh.FMesh, args []string) { s.cmdScript(args) })

	s.MeshCommands[Run] = NewMeshCommandDescriptorWithArgs(
		"run a named scenario, e.g. 'run breakfast'",
		func(_ *fmesh.FMesh, args []string) { s.cmdRun(args) })

	s.MeshCommands[Scripts] = NewMeshCommandDescriptor(
		"list named scenarios", func(_ *fmesh.FMesh) { s.cmdScripts() })

	s.MeshCommands[ListPrograms] = NewMeshCommandDescriptor(
		"list scenarios currently running", func(_ *fmesh.FMesh) { s.cmdPrograms() })

	s.MeshCommands[Stop] = NewMeshCommandDescriptorWithArgs(
		"stop a running scenario by id, or 'stop all'",
		func(_ *fmesh.FMesh, args []string) { s.cmdStop(args) })

	s.MeshCommands[Load] = NewMeshCommandDescriptorWithArgs(
		"read commands from a file, one per line",
		func(_ *fmesh.FMesh, args []string) { s.cmdLoad(args) })
}

// splitDurationAndCommand peels a leading duration off a command line.
func splitDurationAndCommand(args []string, usage string) (duration string, cmd Command, err error) {
	if len(args) < 2 {
		return "", "", fmt.Errorf("usage: %s", usage)
	}
	return args[0], Command(strings.Join(args[1:], " ")), nil
}

func (s *Simulation) cmdEvery(args []string) {
	// An optional trailing "x5" limits the number of runs.
	times := Forever
	if len(args) > 0 {
		if last := args[len(args)-1]; strings.HasPrefix(last, "x") {
			parsed, err := strconv.Atoi(strings.TrimPrefix(last, "x"))
			if err != nil || parsed <= 0 {
				fmt.Printf("invalid repeat count %q (use e.g. 'x5')\n", last)
				return
			}
			times, args = parsed, args[:len(args)-1]
		}
	}

	interval, cmd, err := splitDurationAndCommand(args, "every <duration> <command...> [x<count>]")
	if err != nil {
		fmt.Println(err)
		return
	}

	d, err := ParseSimDuration(interval)
	if err != nil {
		fmt.Printf("invalid interval %q: %v\n", interval, err)
		return
	}

	job, err := s.Scheduler.Every(s.Now(), d, cmd, times)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("scheduled", job.Describe(s.Now()))
}

func (s *Simulation) cmdAfter(args []string) {
	delay, cmd, err := splitDurationAndCommand(args, "after <duration> <command...>")
	if err != nil {
		fmt.Println(err)
		return
	}

	d, err := ParseSimDuration(delay)
	if err != nil {
		fmt.Printf("invalid delay %q: %v\n", delay, err)
		return
	}

	job := s.Scheduler.At(s.Now()+d, cmd)
	fmt.Println("scheduled", job.Describe(s.Now()))
}

func (s *Simulation) cmdAt(args []string) {
	when, cmd, err := splitDurationAndCommand(args, "at <sim time> <command...>")
	if err != nil {
		fmt.Println(err)
		return
	}

	d, err := ParseSimDuration(when)
	if err != nil {
		fmt.Printf("invalid time %q: %v\n", when, err)
		return
	}

	if d < s.Now() {
		fmt.Printf("simulated time %s has already passed (now %s)\n",
			FormatSimDuration(d), FormatSimDuration(s.Now()))
		return
	}

	job := s.Scheduler.At(d, cmd)
	fmt.Println("scheduled", job.Describe(s.Now()))
}

func (s *Simulation) cmdJobs() {
	jobs := s.Scheduler.Jobs()
	if len(jobs) == 0 {
		fmt.Println("nothing scheduled")
		return
	}

	now := s.Now()
	fmt.Printf("scheduled commands (simulated time now %s):\n", FormatSimDuration(now))
	for _, job := range jobs {
		fmt.Println("  " + job.Describe(now))
	}
}

func (s *Simulation) cmdCancel(args []string) {
	if len(args) != 1 {
		fmt.Println("usage: cancel <id>   or 'cancel all'")
		return
	}

	if args[0] == "all" {
		fmt.Printf("cancelled %d scheduled command(s)\n", s.Scheduler.CancelAll())
		return
	}

	id, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Printf("invalid job id %q\n", args[0])
		return
	}
	if !s.Scheduler.Cancel(id) {
		fmt.Printf("no scheduled command with id %d\n", id)
		return
	}
	fmt.Printf("cancelled job %d\n", id)
}

func (s *Simulation) cmdScript(args []string) {
	if len(args) < 2 {
		fmt.Println("usage: script <name> <step>; <step>; ...")
		return
	}

	name := args[0]
	steps, err := ParseSteps(strings.Join(args[1:], " "))
	if err != nil {
		fmt.Println("could not read scenario:", err)
		return
	}

	s.Programs.Define(name, steps)
	fmt.Printf("defined %q: %s\n", name, FormatSteps(steps))
}

func (s *Simulation) cmdRun(args []string) {
	if len(args) != 1 {
		fmt.Println("usage: run <name>")
		return
	}

	program, err := s.Programs.StartNamed(args[0])
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println("running", program.Describe(s.Now()))
}

func (s *Simulation) cmdScripts() {
	names := s.Programs.Defined()
	if len(names) == 0 {
		fmt.Println("no scenarios defined")
		return
	}

	fmt.Println("defined scenarios:")
	for _, name := range names {
		steps, _ := s.Programs.Steps(name)
		fmt.Printf("  %s: %s\n", name, FormatSteps(steps))
	}
}

func (s *Simulation) cmdPrograms() {
	running := s.Programs.Running()
	if len(running) == 0 {
		fmt.Println("no scenarios running")
		return
	}

	now := s.Now()
	fmt.Printf("running scenarios (simulated time now %s):\n", FormatSimDuration(now))
	for _, program := range running {
		fmt.Println("  " + program.Describe(now))
	}
}

func (s *Simulation) cmdStop(args []string) {
	if len(args) != 1 {
		fmt.Println("usage: stop <id>   or 'stop all'")
		return
	}

	if args[0] == "all" {
		fmt.Printf("stopped %d scenario(s)\n", s.Programs.StopAll())
		return
	}

	id, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Printf("invalid scenario id %q\n", args[0])
		return
	}
	if !s.Programs.Stop(id) {
		fmt.Printf("no scenario running with id %d\n", id)
		return
	}
	fmt.Printf("stopped scenario %d\n", id)
}

// cmdLoad reads commands from a file, one per line, so a scenario can live in
// version control rather than in shell history.
func (s *Simulation) cmdLoad(args []string) {
	if len(args) != 1 {
		fmt.Println("usage: load <file>")
		return
	}

	file, err := os.Open(args[0])
	if err != nil {
		fmt.Println("could not open file:", err)
		return
	}
	defer file.Close()

	var loaded int
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Dispatched rather than queued on cmdChan: we are already on the
		// simulation goroutine, and the channel is owned by the command source.
		if s.dispatchCommand(Command(line)) {
			return
		}
		loaded++
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("could not read file:", err)
		return
	}
	fmt.Printf("ran %d command(s) from %s\n", loaded, args[0])
}
