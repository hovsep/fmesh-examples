package simtime

import (
	"strconv"
	"time"
)

// Pacer holds a simulation back so that simulated time passes at a chosen
// multiple of wall-clock time.
//
// Without it a loop advances as fast as the CPU allows, so a simulated hour can
// elapse in well under a second. That is what tests and batch runs want, but it
// makes a simulation impossible to watch or interact with.
//
// The pacer is told how much simulated time each advance actually covered
// rather than being configured with a fixed step, so it paces a variable-step
// engine (one advance = the next event, minutes or days away) as correctly as a
// fixed-step one.
//
// It is not goroutine-safe: it belongs to the loop that drives the simulation.
type Pacer struct {
	factor     float64       // simulated seconds per wall-clock second; <= 0 means uncapped
	nextStepAt time.Time     // wall-clock time the next advance is due
	budget     time.Duration // wall-clock cost of the last advance, for reporting
}

// Uncapped disables pacing, letting the simulation run flat out.
const Uncapped = 0.0

// napDuration bounds a single wait, so a loop that is holding off still reacts
// promptly to whatever else it has to do (draining commands, shutting down).
const napDuration = 5 * time.Millisecond

// maxCatchUpSteps is how many advances the pacer may fall behind before it
// rebases instead of making up the shortfall.
//
// Some slack absorbs ordinary jitter — scheduler noise, an occasionally slow
// advance — without the pace drifting. Beyond it we are genuinely not keeping
// up, and running the backlog would fire a burst of advances as fast as the CPU
// allows: exactly the lurch pacing exists to prevent. The bound scales with the
// step budget rather than being an absolute duration, since at a millisecond
// per step, tolerating a fixed second would mean bursting a thousand steps.
const maxCatchUpSteps = 4

// New returns an uncapped pacer.
func New() *Pacer { return &Pacer{factor: Uncapped} }

// Factor returns the current simulated seconds per wall-clock second
// (0 = uncapped).
func (p *Pacer) Factor() float64 { return p.factor }

// SetFactor changes the pace: 1 runs the simulation in real time, 60 runs a
// simulated minute per wall second, Uncapped removes the limit.
func (p *Pacer) SetFactor(factor float64) {
	p.factor = factor
	p.Reset()
}

// Enabled reports whether pacing is in effect.
func (p *Pacer) Enabled() bool { return p.factor > 0 }

// Reset drops the accumulated schedule so the next advance is due immediately.
// Call it whenever wall-clock time passes without the simulation running (a
// pause), so it does not try to catch up on time it was never meant to simulate.
func (p *Pacer) Reset() {
	p.nextStepAt = time.Time{}
	p.budget = 0
}

// Ready reports whether the next advance is due. When it is not, the caller
// should Nap and re-check rather than sleeping the whole interval, so it stays
// responsive while waiting.
func (p *Pacer) Ready() bool {
	if !p.Enabled() {
		return true
	}
	return !time.Now().Before(p.nextStepAt)
}

// Nap waits a bounded slice of the time remaining before the next advance.
func (p *Pacer) Nap() {
	wait := min(time.Until(p.nextStepAt), napDuration)
	if wait > 0 {
		time.Sleep(wait)
	}
}

// Advanced records that the simulation just moved simDelta of simulated time
// and schedules when the next advance may run. An advance that moved no
// simulated time costs no wall-clock time either.
func (p *Pacer) Advanced(simDelta time.Duration) {
	if !p.Enabled() || simDelta <= 0 {
		return
	}

	p.budget = time.Duration(float64(simDelta) / p.factor)
	now := time.Now()
	if p.nextStepAt.IsZero() || now.Sub(p.nextStepAt) > maxCatchUpSteps*p.budget {
		// Either the first advance, or we have fallen far enough behind that
		// catching up would mean a burst. Rebase on now: the simulation runs
		// slower than asked rather than lurching.
		p.nextStepAt = now
	}
	p.nextStepAt = p.nextStepAt.Add(p.budget)
}

// WallTimePerStep reports what the last advance was allowed to cost in
// wall-clock time. It is zero until a paced advance has happened.
func (p *Pacer) WallTimePerStep() time.Duration { return p.budget }

// Describe renders the current pace for people to read.
func (p *Pacer) Describe() string {
	if !p.Enabled() {
		return "max (uncapped)"
	}
	if p.budget == 0 {
		return formatFactor(p.factor) + " real time"
	}
	return formatFactor(p.factor) + " real time (" + FormatDuration(p.budget) + " of wall clock per step)"
}

func formatFactor(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64) + "x"
}
