// Package sim and its subpackages are small libraries for building simulations
// out of an fmesh. They are shared by the examples next to them in
// simulation/, and are not themselves an example.
//
// The mesh defines what is being simulated: components hold the state, signals
// carry everything between them, and a mesh run computes the next state. What
// differs between kinds of simulation is only how time advances — and that is
// the one thing this package abstracts, as [Engine].
//
// Everything else is engine-agnostic and lives in a subpackage of its own:
//
//   - simtime  simulated time: a clock, duration parsing that understands days,
//     and a pacer that ties simulated time to wall-clock time.
//   - command  text commands: a line, a registry with grouped help, and the
//     sources that feed lines in (stdin, a script, a UI).
//   - schedule a timeline of commands due at simulated times: one-shot,
//     repeating, or multi-step scenarios.
//   - sink     where telemetry goes: a channel, stdout, or nothing.
//   - session  the interactive driver — it runs an [Engine] in a loop, applies
//     commands between advances, paces, and publishes. It has no
//     dependency on fmesh at all.
//
// Engines live beside each other, one per paradigm:
//
//   - stepsim  fixed-step: one mesh run per step, each worth a constant dt.
//     Use it for continuous processes sampled at a fixed rate.
//
// Planned, and the reason the [Engine] seam exists: des (next-event advance,
// driven by schedule.Timeline's due times), sysdyn (stock-and-flow components
// integrated by stepsim), montecarlo (n independent replications of any engine).
//
// A simulation is therefore an engine plus a session:
//
//	eng := stepsim.New(mesh, 10*time.Millisecond)
//	s := session.New(eng)
//	s.Commands.Add(command.Command{Name: "ping", Description: "…", Run: …})
//	if err := s.Run(); err != nil { … }
package sim
