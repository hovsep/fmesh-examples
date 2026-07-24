package step_sim

import (
	"fmt"
	"slices"
	"time"
)

// Forever marks a repeating job with no limit on how many times it runs.
const Forever = -1

// Job is a command queued to run at a point in simulated time, optionally
// repeating.
type Job struct {
	ID      int
	Command Command

	// DueAt is the simulated time at which the command next runs.
	DueAt time.Duration

	// Every is the repeat interval; zero means the job runs once.
	Every time.Duration

	// Remaining counts the runs left, or Forever.
	Remaining int
}

// Repeats reports whether the job runs more than once.
func (j Job) Repeats() bool { return j.Every > 0 }

// Scheduler runs commands at chosen points in simulated time.
//
// It is deliberately not goroutine-safe: per the Simulation.Run concurrency
// invariant it is only ever touched from the simulation goroutine, both when
// commands register jobs and when the loop collects the due ones.
type Scheduler struct {
	nextID int
	jobs   []*Job
}

func NewScheduler() *Scheduler {
	return &Scheduler{}
}

// At queues a command to run once, at the given simulated time.
func (s *Scheduler) At(due time.Duration, cmd Command) *Job {
	s.nextID++
	job := &Job{ID: s.nextID, Command: cmd, DueAt: due, Remaining: 1}
	s.jobs = append(s.jobs, job)
	return job
}

// Every queues a command to run repeatedly. The first run is one interval from
// now rather than immediately, so "every 1d excretion:defecate" does not fire the
// moment it is typed.
func (s *Scheduler) Every(now, interval time.Duration, cmd Command, times int) (*Job, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("repeat interval must be positive, got %v", interval)
	}

	s.nextID++
	job := &Job{
		ID:        s.nextID,
		Command:   cmd,
		DueAt:     now + interval,
		Every:     interval,
		Remaining: times,
	}
	s.jobs = append(s.jobs, job)
	return job, nil
}

// Cancel removes a job, reporting whether it existed.
func (s *Scheduler) Cancel(id int) bool {
	for i, job := range s.jobs {
		if job.ID == id {
			s.jobs = slices.Delete(s.jobs, i, i+1)
			return true
		}
	}
	return false
}

// CancelAll drops every job and reports how many there were.
func (s *Scheduler) CancelAll() int {
	count := len(s.jobs)
	s.jobs = nil
	return count
}

// Jobs returns a snapshot of the queue, soonest first.
func (s *Scheduler) Jobs() []Job {
	jobs := make([]Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, *job)
	}
	slices.SortFunc(jobs, func(a, b Job) int { return int(a.DueAt - b.DueAt) })
	return jobs
}

// Due collects the commands that have come due, advancing repeats and dropping
// jobs that have run their course.
//
// A job that fell behind (because simulated time jumped further than one
// interval) runs once and is rescheduled from now, rather than firing repeatedly
// to make up the missed occurrences: a day's worth of skipped meals should not
// arrive all at once.
func (s *Scheduler) Due(now time.Duration) []Command {
	var due []Command
	remaining := s.jobs[:0]

	for _, job := range s.jobs {
		if job.DueAt > now {
			remaining = append(remaining, job)
			continue
		}

		due = append(due, job.Command)

		if job.Remaining > 0 {
			job.Remaining--
		}
		if !job.Repeats() || job.Remaining == 0 {
			continue // drop it
		}

		job.DueAt = now + job.Every
		remaining = append(remaining, job)
	}

	s.jobs = remaining
	return due
}

// Describe renders a job for the jobs listing.
func (j Job) Describe(now time.Duration) string {
	when := FormatSimDuration(j.DueAt - now)
	if !j.Repeats() {
		return fmt.Sprintf("[%d] in %s: %s", j.ID, when, j.Command)
	}

	times := "forever"
	if j.Remaining > 0 {
		times = fmt.Sprintf("%d more", j.Remaining)
	}
	return fmt.Sprintf("[%d] every %s (next in %s, %s): %s",
		j.ID, FormatSimDuration(j.Every), when, times, j.Command)
}
