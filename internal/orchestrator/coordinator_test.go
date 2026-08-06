package orchestrator

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type blockingScanRunner struct {
	started  chan struct{}
	release  chan struct{}
	triggers []string
	mu       sync.Mutex
}

func (r *blockingScanRunner) Run(ctx context.Context, trigger string) (ScanResult, error) {
	r.mu.Lock()
	r.triggers = append(r.triggers, trigger)
	r.mu.Unlock()
	select {
	case r.started <- struct{}{}:
	default:
	}
	select {
	case <-r.release:
		return ScanResult{Trigger: trigger}, nil
	case <-ctx.Done():
		return ScanResult{}, ctx.Err()
	}
}

func TestCoordinatedRunnerRejectsOverlappingScans(t *testing.T) {
	underlying := &blockingScanRunner{started: make(chan struct{}, 1), release: make(chan struct{})}
	runner := NewCoordinatedRunner(underlying)
	firstDone := make(chan error, 1)
	go func() {
		_, err := runner.Run(context.Background(), "scheduled")
		firstDone <- err
	}()
	<-underlying.started

	if _, err := runner.Run(context.Background(), "manual"); !errors.Is(err, ErrScanInProgress) {
		t.Fatalf("expected in-progress error, got %v", err)
	}

	close(underlying.release)
	if err := <-firstDone; err != nil {
		t.Fatalf("complete scheduled scan: %v", err)
	}
	if len(underlying.triggers) != 1 || underlying.triggers[0] != "scheduled" {
		t.Fatalf("unexpected underlying calls: %#v", underlying.triggers)
	}
}
