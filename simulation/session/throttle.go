package session

import "time"

// Throttle rate-limits how often telemetry is published, decoupling how fast a
// simulation runs from how fast anything watching it needs to be told.
//
// The interval is wall-clock time, not simulated time: it exists for the sake
// of the consumer (a screen that repaints, a file that would otherwise grow
// without bound), and consumers live in the real world.
//
// It is not goroutine-safe: it belongs to the session loop.
type Throttle struct {
	interval time.Duration // 0 (or less) publishes every time
	last     time.Time
}

func NewThrottle(interval time.Duration) *Throttle {
	return &Throttle{interval: interval}
}

func (t *Throttle) SetInterval(d time.Duration) { t.interval = d }

func (t *Throttle) Interval() time.Duration { return t.interval }

// Allow reports whether to publish now and, if so, records that we did.
// Snapshots produced while throttled are simply dropped; the next allowed one
// carries the then-current state.
func (t *Throttle) Allow() bool {
	if t.interval <= 0 {
		return true
	}

	now := time.Now()
	if now.Sub(t.last) >= t.interval {
		t.last = now
		return true
	}
	return false
}
