package step_sim

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/hovsep/fmesh"
	"github.com/hovsep/fmesh-examples/simulation/step_sim/sink"
	"github.com/hovsep/fmesh/cycle"
)

type MeshCommandMap map[Command]MeshCommandDescriptor

type SimInitFunc func(sim *Simulation)

// Simulation is a wrapper around a mesh
// it runs the mesh in a loop and feeds it with commands from outside (e.g., REPL or another system)
type Simulation struct {
	ctx             context.Context  // Context is used to cancel the simulation
	cmdChan         chan Command     // Channel for commands from outside
	isPaused        bool             // Flag to pause the simulation
	FM              *fmesh.FMesh     // The mesh
	MeshCommands    MeshCommandMap   // Commands that can be executed on the mesh
	AutoPause       bool             // Automatically pause the simulation if nothing happens
	Sink            sink.Sink        // Sink is useful for sending messages to the outside (ui, metrics, etc.)
	PublishThrottle *PublishThrottle // Rate-limits how often state is published to the Sink (0 = every cycle)
	Pacer           *SimPacer        // Paces simulated time against wall-clock time (uncapped by default)

	// SimClock reports elapsed simulated time. Scheduling is expressed in it, so
	// concrete simulations should point this at their own clock; it defaults to
	// wall-clock time since the simulation was created.
	SimClock SimClock

	Scheduler *Scheduler // Commands queued to run at points in simulated time
	Programs  *Programs  // Scenarios: ordered commands with waits between them
}

func NewSimulation(ctx context.Context, fm *fmesh.FMesh, cmdChan chan Command, sink sink.Sink) *Simulation {
	sim := &Simulation{
		ctx:             ctx,
		FM:              fm,
		cmdChan:         cmdChan,
		MeshCommands:    getDefaultMeshCommands(),
		Sink:            sink,
		PublishThrottle: NewPublishThrottle(0), // no throttling by default; publish every cycle
		// Uncapped by default, and inert until a caller declares how much
		// simulated time a tick represents (only the concrete simulation knows).
		Pacer:     NewSimPacer(0),
		SimClock:  wallClockSince(time.Now()),
		Scheduler: NewScheduler(),
		Programs:  NewPrograms(),
	}
	sim.registerSchedulingCommands()
	return sim
}

func getDefaultMeshCommands() MeshCommandMap {
	meshCommands := make(MeshCommandMap)
	// Default commands are handled by the REPL, we add them here just to handle descriptions in one place
	meshCommands[Exit] = NewMeshCommandDescriptor("exit REPL", NoopMeshCommand)
	meshCommands[Pause] = NewMeshCommandDescriptor("pause simulation", NoopMeshCommand)
	meshCommands[Resume] = NewMeshCommandDescriptor("resume simulation", NoopMeshCommand)
	meshCommands[Help] = NewMeshCommandDescriptor("show this help message", func(_ *fmesh.FMesh) {
		showHelp(meshCommands)
	})
	return meshCommands
}

// Init allows initializing the simulation before the simulation starts,
// e.g., adding custom commands or manipulating the mesh before it starts running
func (s *Simulation) Init(initFunc func(sim *Simulation)) *Simulation {
	initFunc(s)
	return s
}

// Run starts the simulation loop.
//
// Concurrency invariant: the mesh is only ever touched from this single
// goroutine. Both command handling and FM.Run() happen here, serialized,
// so no locking is required around the mesh or component state. New
// simulations must mutate the mesh exclusively via commands sent on cmdChan,
// never directly from the sink, UI, or other goroutines.
func (s *Simulation) Run() {
	for {
		// Drain all pending commands without blocking
		drainCommands := true
		for drainCommands {
			select {
			case <-s.ctx.Done():
				fmt.Println("Shutting down simulation...")
				return
			case cmd, ok := <-s.cmdChan:
				if !ok {
					fmt.Println("Command channel closed, shutting down simulation...")
					return
				}
				if s.dispatchCommand(cmd) {
					return
				}
			default:
				// No more commands in the channel, stop draining
				drainCommands = false
			}
		}

		// While paused, block until a command arrives (or shutdown) instead of
		// busy-waiting, so we react immediately and consume no CPU.
		if s.isPaused {
			select {
			case <-s.ctx.Done():
				fmt.Println("Shutting down simulation...")
				return
			case cmd, ok := <-s.cmdChan:
				if !ok {
					fmt.Println("Command channel closed, shutting down simulation...")
					return
				}
				if s.dispatchCommand(cmd) {
					return
				}
			}
			continue
		}

		// Hold off until the next tick is due, re-entering the loop rather than
		// sleeping the whole interval, so commands keep being drained while we wait.
		if !s.Pacer.Ready() {
			s.Pacer.Nap()
			continue
		}

		// Fire anything simulated time has brought due. This runs here, on the
		// simulation goroutine and between mesh runs, so scheduled commands
		// reach the mesh exactly the way typed ones do.
		if stop := s.runDueCommands(); stop {
			return
		}

		// Run a single simulation cycle
		runResult, err := s.FM.Run()
		s.Pacer.Advance()
		if err != nil {
			// A failed cycle must not kill the simulation goroutine: if it did,
			// nothing would drain cmdChan and the REPL would block forever on its
			// next command. Pause instead and keep looping, so the user can
			// inspect, adjust, resume, or exit.
			fmt.Println("Simulation cycle finished with error:", err)
			s.Pause()
			fmt.Println("Type 'resume' to continue or 'exit' to quit.")
			continue
		}

		s.MaybeAutoPause(runResult)
	}
}

// runDueCommands executes everything the scheduler and any running programs have
// brought due at the current simulated time.
func (s *Simulation) runDueCommands() (stop bool) {
	now := s.Now()
	for _, cmd := range append(s.Scheduler.Due(now), s.Programs.Advance(now)...) {
		if s.dispatchCommand(cmd) {
			return true
		}
	}
	return false
}

// dispatchCommand executes a single command and returns true if the
// simulation loop should stop.
func (s *Simulation) dispatchCommand(cmd Command) (stop bool) {
	switch cmd {
	case Pause:
		s.Pause()
	case Resume:
		s.Resume()
	case Exit:
		fmt.Println("Exiting simulation...")
		return true
	default:
		// A line with step separators is a scenario, not a command: run it as an
		// anonymous program so its waits suspend only itself.
		if isProgramLine(cmd) {
			s.startAnonymousProgram(cmd)
			return false
		}
		s.handleCommand(cmd)
	}
	return false
}

// isProgramLine reports whether a line should be run as a program.
//
// Commands that take a raw line containing separators (defining a program) are
// excluded, or defining one would run it instead.
func isProgramLine(cmd Command) bool {
	if !strings.Contains(string(cmd), StepSeparator) {
		return false
	}
	fields := strings.Fields(string(cmd))
	return len(fields) > 0 && fields[0] != string(Script)
}

func (s *Simulation) startAnonymousProgram(cmd Command) {
	steps, err := ParseSteps(string(cmd))
	if err != nil {
		fmt.Println("could not read scenario:", err)
		return
	}

	program := s.Programs.Start("", steps)
	fmt.Printf("running scenario [%d]: %s\n", program.ID, FormatSteps(steps))
}

func (s *Simulation) MaybeAutoPause(runResult *fmesh.RuntimeInfo) {
	if !s.AutoPause {
		return
	}

	// Auto-pause if nothing is happening
	if runResult.Cycles.Count(func(c *cycle.Cycle) bool {
		return c.HasActivatedComponents()
	}) == 0 {
		fmt.Println("Simulation does not progress and will be paused (nothing happens in your mesh)")
		s.Pause()
	}
}

func (s *Simulation) Pause() {
	fmt.Println("Simulation paused")
	s.isPaused = true
}

func (s *Simulation) Resume() {
	fmt.Println("Simulation resumed")
	s.isPaused = false
	// Wall-clock time passed while we were paused but simulated time did not,
	// so drop the schedule instead of racing to make up ticks that were never
	// meant to run.
	s.Pacer.Reset()
}

// handleCommand executes a valid command. The command line is split into a
// name and its arguments, so commands like "rate 100ms" are dispatched by their
// first token and receive the rest as args.
func (s *Simulation) handleCommand(cmd Command) {
	fields := strings.Fields(string(cmd))
	if len(fields) == 0 {
		return
	}

	name, args := Command(fields[0]), fields[1:]
	cmdDescriptor, ok := s.MeshCommands[name]
	if !ok {
		fmt.Printf("Unknown command: %v \n", cmd)
		return
	}
	cmdDescriptor.RunWithMesh(s.FM, args)
}

func (s *Simulation) SendCommand(cmd Command) {
	s.cmdChan <- cmd
}

// CommandNames returns every registered command name, sorted. Front ends use it
// for completion and listings, so they never go out of step with what the
// simulation actually accepts.
func (s *Simulation) CommandNames() []string {
	names := make([]string, 0, len(s.MeshCommands))
	for cmd := range s.MeshCommands {
		names = append(names, string(cmd))
	}
	slices.Sort(names)
	return names
}

func showHelp(meshCommands MeshCommandMap) {
	fmt.Println("Available commands:")

	for _, cmd := range slices.Sorted(maps.Keys(meshCommands)) {
		fmt.Printf("  %s - %s\n", cmd, meshCommands[cmd].Description)
	}
}
