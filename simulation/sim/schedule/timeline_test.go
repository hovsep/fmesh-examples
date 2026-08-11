package schedule

import (
	"testing"
	"time"

	"github.com/hovsep/fmesh-examples/simulation/sim/command"
)

// --- Scheduled jobs (formerly Scheduler) ---------------------------------

func TestTimeline_OneShotFiresOnceWhenDue(t *testing.T) {
	tl := New()
	tl.After(time.Hour, "valve:open 250")

	if due := tl.Advance(30 * time.Minute); len(due) != 0 {
		t.Fatalf("job fired early: %v", due)
	}

	due := tl.Advance(time.Hour)
	if len(due) != 1 || due[0] != "valve:open 250" {
		t.Fatalf("expected the job at its due time, got %v", due)
	}

	if due := tl.Advance(2 * time.Hour); len(due) != 0 {
		t.Fatalf("one-shot job fired again: %v", due)
	}
	if len(tl.Jobs()) != 0 {
		t.Fatal("a spent one-shot job is still queued")
	}
}

func TestTimeline_RepeatDoesNotFireImmediately(t *testing.T) {
	tl := New()
	// "every 1d" typed at time zero should not also mean "and right now".
	if _, err := tl.Every(0, 24*time.Hour, "tank:drain", Forever); err != nil {
		t.Fatal(err)
	}

	if due := tl.Advance(0); len(due) != 0 {
		t.Fatalf("repeating job fired at the moment it was scheduled: %v", due)
	}
	if due := tl.Advance(24 * time.Hour); len(due) != 1 {
		t.Fatalf("repeating job did not fire after one interval: %v", due)
	}
}

func TestTimeline_RepeatKeepsGoing(t *testing.T) {
	tl := New()
	if _, err := tl.Every(0, time.Hour, "valve:open 250", Forever); err != nil {
		t.Fatal(err)
	}

	for hour := 1; hour <= 5; hour++ {
		due := tl.Advance(time.Duration(hour) * time.Hour)
		if len(due) != 1 {
			t.Fatalf("hour %d: expected one command, got %v", hour, due)
		}
	}
	if len(tl.Jobs()) != 1 {
		t.Fatal("an endless job should stay queued")
	}
}

func TestTimeline_RepeatRespectsACount(t *testing.T) {
	tl := New()
	if _, err := tl.Every(0, time.Hour, "heater:on 100", 3); err != nil {
		t.Fatal(err)
	}

	var fired int
	for hour := 1; hour <= 10; hour++ {
		fired += len(tl.Advance(time.Duration(hour) * time.Hour))
	}

	if fired != 3 {
		t.Fatalf("expected exactly 3 runs, got %d", fired)
	}
	if len(tl.Jobs()) != 0 {
		t.Fatal("a job that ran its course is still queued")
	}
}

func TestTimeline_DoesNotReplayMissedOccurrences(t *testing.T) {
	tl := New()
	if _, err := tl.Every(0, time.Hour, "heater:on 500", Forever); err != nil {
		t.Fatal(err)
	}

	// Simulated time jumps a full day past the first due time. Firing 24 runs
	// at once would be worse than useless, so the job should run once and
	// reschedule from here.
	due := tl.Advance(24 * time.Hour)
	if len(due) != 1 {
		t.Fatalf("expected a single catch-up run, got %d", len(due))
	}

	if due := tl.Advance(24*time.Hour + 30*time.Minute); len(due) != 0 {
		t.Fatalf("job fired before a full interval had passed: %v", due)
	}
	if due := tl.Advance(25 * time.Hour); len(due) != 1 {
		t.Fatalf("job did not resume on its interval: %v", due)
	}
}

func TestTimeline_RejectsNonPositiveInterval(t *testing.T) {
	tl := New()
	if _, err := tl.Every(0, 0, "valve:open 1", Forever); err == nil {
		t.Fatal("a zero interval would busy-loop; it must be rejected")
	}
	if _, err := tl.Every(0, -time.Hour, "valve:open 1", Forever); err == nil {
		t.Fatal("a negative interval must be rejected")
	}
}

func TestTimeline_CancelJob(t *testing.T) {
	tl := New()
	job := tl.After(time.Hour, "valve:open 250")

	if !tl.Cancel(job.ID) {
		t.Fatal("Cancel reported the job did not exist")
	}
	if tl.Cancel(job.ID) {
		t.Fatal("Cancel reported success twice for the same job")
	}
	if due := tl.Advance(2 * time.Hour); len(due) != 0 {
		t.Fatalf("a cancelled job still fired: %v", due)
	}
}

func TestTimeline_JobsAreListedSoonestFirst(t *testing.T) {
	tl := New()
	tl.After(3*time.Hour, "third")
	tl.After(time.Hour, "first")
	tl.After(2*time.Hour, "second")

	jobs := tl.Jobs()
	want := []command.Line{"first", "second", "third"}
	for i, w := range want {
		if jobs[i].Command() != w {
			t.Fatalf("job %d is %q, want %q; the listing is not ordered by due time", i, jobs[i].Command(), w)
		}
	}
}

// --- Scenarios (formerly Programs) ---------------------------------------

func TestParseSteps(t *testing.T) {
	steps, err := ParseSteps("heater:on 200; wait 1h; fan:start 8 30m")
	if err != nil {
		t.Fatal(err)
	}

	if len(steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(steps))
	}
	if steps[0].Command != "heater:on 200" {
		t.Errorf("step 0 = %q", steps[0].Command)
	}
	if !steps[1].IsWait() || steps[1].Wait != time.Hour {
		t.Errorf("step 1 should be a one-hour wait, got %v", steps[1])
	}
	if steps[2].Command != "fan:start 8 30m" {
		t.Errorf("step 2 = %q", steps[2].Command)
	}
}

func TestParseSteps_Rejects(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"empty line", ""},
		{"only separators", ";;;"},
		{"wait with no duration", "wait"},
		{"wait with too many arguments", "wait 1h 30m"},
		{"wait with a bad duration", "wait banana"},
		{"negative wait", "wait -1h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseSteps(tt.in); err == nil {
				t.Fatalf("ParseSteps(%q) accepted an invalid scenario", tt.in)
			}
		})
	}
}

func TestParseStepsRoundTrips(t *testing.T) {
	line := "heater:on 200; wait 1h; fan:start 8 30m"
	steps, err := ParseSteps(line)
	if err != nil {
		t.Fatal(err)
	}
	if got := FormatSteps(steps); got != line {
		t.Errorf("FormatSteps round-trip = %q, want %q", got, line)
	}
}

func TestTimeline_RunsConsecutiveCommandsAtOnce(t *testing.T) {
	tl := New()
	steps, err := ParseSteps("first; second; third")
	if err != nil {
		t.Fatal(err)
	}
	tl.Start("", steps)

	// Nothing separates these steps, so they should all run in one pass rather
	// than dribbling out one per tick.
	commands := tl.Advance(0)
	if len(commands) != 3 {
		t.Fatalf("expected all 3 commands at once, got %v", commands)
	}
	if len(tl.Scenarios()) != 0 {
		t.Fatal("a finished scenario is still listed as running")
	}
}

func TestTimeline_WaitSuspendsUntilSimTimePasses(t *testing.T) {
	tl := New()
	steps, err := ParseSteps("heater:on 200; wait 1h; fan:start 8 30m")
	if err != nil {
		t.Fatal(err)
	}
	tl.Start("warmup", steps)

	commands := tl.Advance(0)
	if len(commands) != 1 || commands[0] != "heater:on 200" {
		t.Fatalf("expected only the pre-wait command, got %v", commands)
	}

	if commands := tl.Advance(59 * time.Minute); len(commands) != 0 {
		t.Fatalf("scenario resumed before its wait elapsed: %v", commands)
	}

	commands = tl.Advance(time.Hour)
	if len(commands) != 1 || commands[0] != "fan:start 8 30m" {
		t.Fatalf("expected the post-wait command, got %v", commands)
	}
	if len(tl.Scenarios()) != 0 {
		t.Fatal("the scenario should be finished")
	}
}

func TestTimeline_ScenariosRunConcurrently(t *testing.T) {
	tl := New()

	slow, err := ParseSteps("slow-start; wait 2h; slow-end")
	if err != nil {
		t.Fatal(err)
	}
	fast, err := ParseSteps("fast-start; wait 30m; fast-end")
	if err != nil {
		t.Fatal(err)
	}
	tl.Start("slow", slow)
	tl.Start("fast", fast)

	if commands := tl.Advance(0); len(commands) != 2 {
		t.Fatalf("both scenarios should have started, got %v", commands)
	}

	// One scenario's wait must not hold up the other's.
	commands := tl.Advance(30 * time.Minute)
	if len(commands) != 1 || commands[0] != "fast-end" {
		t.Fatalf("expected only the fast scenario to resume, got %v", commands)
	}
	if len(tl.Scenarios()) != 1 {
		t.Fatal("the slow scenario should still be waiting")
	}

	commands = tl.Advance(2 * time.Hour)
	if len(commands) != 1 || commands[0] != "slow-end" {
		t.Fatalf("expected the slow scenario to resume, got %v", commands)
	}
}

func TestTimeline_NamedDefinitions(t *testing.T) {
	tl := New()
	steps, err := ParseSteps("valve:open 250; wait 15m; valve:open 250")
	if err != nil {
		t.Fatal(err)
	}
	tl.Define("hydrate", steps)

	if _, err := tl.StartNamed("nonexistent"); err == nil {
		t.Fatal("StartNamed accepted an unknown name")
	}

	scenario, err := tl.StartNamed("hydrate")
	if err != nil {
		t.Fatal(err)
	}
	if scenario.Label != "hydrate" {
		t.Errorf("scenario name = %q", scenario.Label)
	}

	// A definition is reusable, so running it must not consume it.
	if _, err := tl.StartNamed("hydrate"); err != nil {
		t.Fatalf("a named scenario should be runnable more than once: %v", err)
	}
	if len(tl.Scenarios()) != 2 {
		t.Fatalf("expected 2 running instances, got %d", len(tl.Scenarios()))
	}
}

func TestTimeline_StopScenario(t *testing.T) {
	tl := New()
	steps, err := ParseSteps("start; wait 1h; end")
	if err != nil {
		t.Fatal(err)
	}
	scenario := tl.Start("", steps)
	tl.Advance(0)

	if !tl.Cancel(scenario.ID) {
		t.Fatal("Cancel reported the scenario was not running")
	}
	if commands := tl.Advance(2 * time.Hour); len(commands) != 0 {
		t.Fatalf("a stopped scenario still ran: %v", commands)
	}
}

// --- Jobs and scenarios coexist without colliding ------------------------

func TestTimeline_JobsAndScenariosListSeparately(t *testing.T) {
	tl := New()
	tl.After(time.Hour, "a-job")
	steps, _ := ParseSteps("a; wait 1h; b")
	tl.Start("scenario", steps)

	if len(tl.Jobs()) != 1 {
		t.Fatalf("expected one job, got %d", len(tl.Jobs()))
	}
	if len(tl.Scenarios()) != 1 {
		t.Fatalf("expected one scenario, got %d", len(tl.Scenarios()))
	}

	// StopAllScenarios must leave the job; CancelAllJobs must leave the scenario.
	if n := tl.StopAllScenarios(); n != 1 {
		t.Fatalf("StopAllScenarios removed %d, want 1", n)
	}
	if len(tl.Jobs()) != 1 {
		t.Fatal("stopping scenarios also removed the job")
	}
	if n := tl.CancelAllJobs(); n != 1 {
		t.Fatalf("CancelAllJobs removed %d, want 1", n)
	}
}
