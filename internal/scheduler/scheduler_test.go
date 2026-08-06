package scheduler

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/orchestrator"
)

type fakeRunner struct {
	trigger string
	calls   int
}

func (r *fakeRunner) Run(_ context.Context, trigger string) (orchestrator.ScanResult, error) {
	r.trigger = trigger
	r.calls++
	return orchestrator.ScanResult{RunID: 12, Status: "completed"}, nil
}

func TestNewValidatesScheduleAndTimezone(t *testing.T) {
	runner := &fakeRunner{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if _, err := New("bad", "Europe/Istanbul", runner, logger); err == nil {
		t.Fatal("expected invalid cron schedule to fail")
	}
	if _, err := New("0 9 * * 1", "not/a-timezone", runner, logger); err == nil {
		t.Fatal("expected invalid timezone to fail")
	}
	if _, err := New("0 9 31 2 *", "Europe/Istanbul", runner, logger); err == nil {
		t.Fatal("expected impossible calendar schedule to fail")
	}
	if _, err := New("0 9 * * 1", "Europe/Istanbul", runner, logger); err != nil {
		t.Fatalf("valid schedule: %v", err)
	}
	if _, err := New("*/15 8-18 * 1,6 1-5", "UTC", runner, logger); err != nil {
		t.Fatalf("valid list/range/step schedule: %v", err)
	}
}

func TestCronNextUsesConfiguredTimezoneAndWeekday(t *testing.T) {
	schedule, err := parseCron("0 9 * * 1")
	if err != nil {
		t.Fatalf("parse cron: %v", err)
	}
	istanbul, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		t.Fatalf("load timezone: %v", err)
	}
	after := time.Date(2026, 8, 2, 23, 30, 0, 0, time.UTC).In(istanbul)
	next := schedule.Next(after)
	want := time.Date(2026, 8, 3, 9, 0, 0, 0, istanbul)
	if !next.Equal(want) || next.Location() != istanbul {
		t.Fatalf("next scheduled run = %s (%s), want %s (%s)", next, next.Location(), want, istanbul)
	}
}

func TestRunUsesScheduledTrigger(t *testing.T) {
	runner := &fakeRunner{}
	scheduler, err := New("0 9 * * 1", "Europe/Istanbul", runner, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}
	scheduler.run(context.Background())
	if runner.calls != 1 || runner.trigger != "scheduled" {
		t.Fatalf("unexpected scheduled runner call: %#v", runner)
	}
}

func TestSchedulerStopsWhenContextIsCancelled(t *testing.T) {
	scheduler, err := New("0 9 * * 1", "Europe/Istanbul", &fakeRunner{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("new scheduler: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	scheduler.Start(ctx)
	cancel()
	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	if err := scheduler.Wait(waitCtx); err != nil {
		t.Fatalf("scheduler did not stop: %v", err)
	}
}
