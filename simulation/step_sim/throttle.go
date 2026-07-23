package step_sim

import "time"

// PublishThrottle rate-limits how often snapshots are published to a sink,
// decoupling the simulation's cycle rate from the UI's update rate.
//
// It is intentionally NOT goroutine-safe: per the Simulation.Run concurrency
// invariant, both command handlers and mesh hooks run on the single sim
// goroutine, so no locking is required.
type PublishThrottle struct {
	interval time.Duration // 0 (or less) means publish every cycle
	last     time.Time
}

func NewPublishThrottle(interval time.Duration) *PublishThrottle {
	return &PublishThrottle{interval: interval}
}

func (t *PublishThrottle) SetInterval(d time.Duration) { t.interval = d }

func (t *PublishThrottle) Interval() time.Duration { return t.interval }

// Allow reports whether a snapshot should be published now and, if so, records
// the publish time. The decision is made per whole snapshot (one FM.Run), never
// per line, so a published snapshot is always internally consistent. Snapshots
// produced while throttled are simply dropped; the next allowed run publishes
// the then-current state.
func (t *PublishThrottle) Allow() bool {
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
