package step_sim

import "time"

// SimPacer throttles the simulation loop so that simulated time advances at a
// chosen multiple of wall-clock time.
//
// Without it, Run calls FM.Run() as fast as the CPU allows, so a simulated hour
// can elapse in well under a second. That is what you want for tests and
// batch runs, but it makes the simulation impossible to watch or interact with
// in real time.
//
// Like PublishThrottle, this is intentionally NOT goroutine-safe: per the
// Simulation.Run concurrency invariant both command handlers and the loop
// itself run on the single simulation goroutine, so no locking is required.
type SimPacer struct {
	factor         float64       // simulated seconds per wall-clock second; <= 0 means uncapped
	simTimePerTick time.Duration // simulated time represented by one FM.Run()
	nextTickAt     time.Time     // wall-clock deadline for the next tick
}

// Uncapped disables pacing, letting the loop run flat out (the historical behaviour).
const Uncapped = 0.0

// napDuration bounds a single wait so the loop keeps draining commands promptly
// even when the pace is very slow.
const napDuration = 5 * time.Millisecond

// maxCatchUpTicks is how many tick budgets the pacer may fall behind before it
// rebases instead of making up the shortfall.
//
// Some slack absorbs ordinary jitter (scheduler noise, an occasionally slow
// mesh run) without the pace drifting. Beyond it we are genuinely not keeping
// up, and running the backlog would fire a burst of ticks as fast as the CPU
// allows -- exactly the lurch pacing exists to prevent. The bound has to scale
// with the tick budget rather than being an absolute duration: at 1ms per tick,
// tolerating a fixed second would mean bursting a thousand ticks.
const maxCatchUpTicks = 4

func NewSimPacer(simTimePerTick time.Duration) *SimPacer {
	return &SimPacer{factor: Uncapped, simTimePerTick: simTimePerTick}
}

// Factor returns the current simulated seconds per wall-clock second (0 = uncapped).
func (p *SimPacer) Factor() float64 { return p.factor }

// SetFactor changes the pace. A factor of 1 runs the simulation in real time, 60
// runs a simulated minute per wall second, and Uncapped removes the limit.
func (p *SimPacer) SetFactor(factor float64) {
	p.factor = factor
	p.Reset()
}

// SimTimePerTick returns how much simulated time one tick represents.
func (p *SimPacer) SimTimePerTick() time.Duration { return p.simTimePerTick }

// SetSimTimePerTick tells the pacer how much simulated time one FM.Run()
// represents. Pacing cannot work until this is set, since the whole conversion
// from simulated to wall-clock time depends on it.
func (p *SimPacer) SetSimTimePerTick(d time.Duration) {
	p.simTimePerTick = d
	p.Reset()
}

// Reset drops the accumulated schedule so the next tick is due immediately.
// Call it whenever wall-clock time passes without the loop running (a pause) or
// when the pace changes, so the loop does not try to catch up on time it was
// never meant to simulate.
func (p *SimPacer) Reset() { p.nextTickAt = time.Time{} }

// Enabled reports whether pacing is actually in effect.
func (p *SimPacer) Enabled() bool { return p.factor > 0 && p.simTimePerTick > 0 }

// WallTimePerTick returns how long one tick should take in wall-clock terms.
func (p *SimPacer) WallTimePerTick() time.Duration {
	if !p.Enabled() {
		return 0
	}
	return time.Duration(float64(p.simTimePerTick) / p.factor)
}

// Ready reports whether the next tick is due. When it is not, the caller should
// Nap and re-check rather than sleeping the whole interval, so that commands
// keep being drained while the loop waits.
func (p *SimPacer) Ready() bool {
	if !p.Enabled() {
		return true
	}
	return !time.Now().Before(p.nextTickAt)
}

// Advance records that a tick has just run and schedules the next one.
func (p *SimPacer) Advance() {
	if !p.Enabled() {
		return
	}

	now := time.Now()
	budget := p.WallTimePerTick()
	if p.nextTickAt.IsZero() || now.Sub(p.nextTickAt) > maxCatchUpTicks*budget {
		// Either the first tick, or we have fallen far enough behind that
		// catching up would mean a burst. Rebase on now instead: the simulation
		// runs slower than requested rather than lurching.
		p.nextTickAt = now
	}
	p.nextTickAt = p.nextTickAt.Add(budget)
}

// Nap waits a bounded slice of the remaining time before the next tick is due.
func (p *SimPacer) Nap() {
	wait := min(time.Until(p.nextTickAt), napDuration)
	if wait > 0 {
		time.Sleep(wait)
	}
}
