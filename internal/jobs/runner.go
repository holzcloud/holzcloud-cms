// Package jobs runs periodic background work.
//
// It replaces the hand-rolled tickers that were scattered across the codebase,
// each with its own stop channel and its own shutdown bug. One runner means one
// place that honours the shutdown context.
package jobs

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Job is a named piece of periodic work.
type Job struct {
	Name  string
	Every time.Duration
	// RunAtStart runs the job once immediately instead of waiting a full
	// interval — useful for work whose result is reported by /readyz.
	RunAtStart bool
	Fn         func(context.Context) error
}

// Runner executes jobs until its context is cancelled.
type Runner struct {
	jobs []Job
	wg   sync.WaitGroup
}

// New creates a runner for the given jobs.
func New(jobs ...Job) *Runner { return &Runner{jobs: jobs} }

// Start launches every job. It returns immediately; use Wait to block until all
// of them have stopped.
func (r *Runner) Start(ctx context.Context) {
	for _, job := range r.jobs {
		if job.Fn == nil || job.Every <= 0 {
			continue
		}
		r.wg.Add(1)
		go r.run(ctx, job)
	}
}

func (r *Runner) run(ctx context.Context, job Job) {
	defer r.wg.Done()

	if job.RunAtStart {
		r.invoke(ctx, job)
	}

	ticker := time.NewTicker(job.Every)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.invoke(ctx, job)
		case <-ctx.Done():
			slog.Debug("job stopped", "job", job.Name)
			return
		}
	}
}

// invoke runs one iteration, keeping a failure from taking the runner down: a
// job that errors once must still run again next interval.
func (r *Runner) invoke(ctx context.Context, job Job) {
	defer func() {
		if rec := recover(); rec != nil {
			slog.Error("job panicked", "job", job.Name, "err", rec)
		}
	}()
	start := time.Now()
	if err := job.Fn(ctx); err != nil {
		slog.Error("job failed", "job", job.Name, "err", err,
			"duration_ms", time.Since(start).Milliseconds())
		return
	}
	slog.Debug("job finished", "job", job.Name,
		"duration_ms", time.Since(start).Milliseconds())
}

// Wait blocks until every job has returned.
func (r *Runner) Wait() { r.wg.Wait() }
