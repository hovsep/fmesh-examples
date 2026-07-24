package helper

// Process meters a quantity out over time at a fixed rate. It models any bodily
// action that is not instantaneous: drinking a glass, eating a meal, smoking a
// cigarette. Because the rate is fixed, a larger quantity simply takes
// proportionally longer -- 500 mL of water takes ten times as long as 50 mL.
//
// It is the reusable core of a controller's "still doing X" state. A controller
// keeps a set of these and, each tick, asks every one for the portion it
// delivers, so several actions can overlap (drinking while walking).
type Process struct {
	// Kind names what is being delivered (e.g. "water_ml"). A controller sums
	// deliveries by kind to decide what to emit.
	Kind string

	// Remaining is the quantity still to deliver, in the kind's own unit.
	Remaining float64

	// RatePerSec is how much is delivered per simulated second.
	RatePerSec float64
}

// Advance delivers this tick's portion and returns it, reducing Remaining. It
// never delivers more than is left, so the total delivered equals the quantity
// the process started with.
func (p *Process) Advance(dt float64) float64 {
	delivered := min(p.RatePerSec*dt, p.Remaining)
	if delivered < 0 {
		delivered = 0
	}
	p.Remaining -= delivered
	return delivered
}

// Done reports whether the process has fully delivered.
func (p *Process) Done() bool { return p.Remaining <= 0 }

// ProcessSet is a controller's collection of in-flight processes.
//
// It is a small value held in component state; the zero value is ready to use.
type ProcessSet struct {
	active []*Process
}

// Start adds a process, ignoring non-positive quantities or rates (nothing to do).
func (s *ProcessSet) Start(p *Process) {
	if p == nil || p.Remaining <= 0 || p.RatePerSec <= 0 {
		return
	}
	s.active = append(s.active, p)
}

// Advance runs every active process for one tick and returns the total delivered
// per kind, dropping any that have finished.
func (s *ProcessSet) Advance(dt float64) map[string]float64 {
	if len(s.active) == 0 {
		return nil
	}

	delivered := make(map[string]float64)
	kept := s.active[:0]
	for _, p := range s.active {
		delivered[p.Kind] += p.Advance(dt)
		if !p.Done() {
			kept = append(kept, p)
		}
	}
	s.active = kept
	return delivered
}

// Active reports how many processes are still running. Handy for observability
// and tests ("is Leon still drinking?").
func (s *ProcessSet) Active() int { return len(s.active) }

// RemainingOf returns the quantity left across all active processes of a kind.
func (s *ProcessSet) RemainingOf(kind string) float64 {
	var total float64
	for _, p := range s.active {
		if p.Kind == kind {
			total += p.Remaining
		}
	}
	return total
}
