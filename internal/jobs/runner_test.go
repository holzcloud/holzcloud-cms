package jobs

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunnerRunsAtStartAndOnInterval(t *testing.T) {
	var runs atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := New(Job{
		Name:       "zaehler",
		Every:      10 * time.Millisecond,
		RunAtStart: true,
		Fn:         func(context.Context) error { runs.Add(1); return nil },
	})
	r.Start(ctx)

	deadline := time.After(2 * time.Second)
	for runs.Load() < 3 {
		select {
		case <-deadline:
			t.Fatalf("job ran only %d times", runs.Load())
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	r.Wait()
}

// A job that fails must not take the runner down — it has to run again next
// interval, which is the difference between a transient error and a dead
// background task.
func TestRunnerSurvivesFailureAndPanic(t *testing.T) {
	var errRuns, panicRuns atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := New(
		Job{Name: "faellt-aus", Every: 5 * time.Millisecond, RunAtStart: true,
			Fn: func(context.Context) error { errRuns.Add(1); return errors.New("kaputt") }},
		Job{Name: "panik", Every: 5 * time.Millisecond, RunAtStart: true,
			Fn: func(context.Context) error { panicRuns.Add(1); panic("boom") }},
	)
	r.Start(ctx)

	deadline := time.After(2 * time.Second)
	for errRuns.Load() < 2 || panicRuns.Load() < 2 {
		select {
		case <-deadline:
			t.Fatalf("failing job ran %d times, panicking job %d times", errRuns.Load(), panicRuns.Load())
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	r.Wait()
}

// Every job must stop with the shutdown context; the hand-rolled tickers this
// replaces each had their own stop channel and their own way of getting it wrong.
func TestRunnerStopsWithTheContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := New(Job{
		Name:  "langlaeufer",
		Every: time.Millisecond,
		Fn:    func(context.Context) error { return nil },
	})
	r.Start(ctx)
	cancel()

	done := make(chan struct{})
	go func() { r.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not stop when its context was cancelled")
	}
}

func TestRunnerIgnoresIncompleteJobs(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// No function and a zero interval must simply be skipped, not panic.
	r := New(
		Job{Name: "ohne-funktion", Every: time.Millisecond},
		Job{Name: "ohne-intervall", Fn: func(context.Context) error { return nil }},
	)
	r.Start(ctx)
	cancel()
	r.Wait()
}
