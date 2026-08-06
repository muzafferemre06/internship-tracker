package orchestrator

import (
	"context"
	"errors"
	"sync"
)

var ErrScanInProgress = errors.New("a scan is already in progress")

// ScanRunner is the common boundary used by HTTP and scheduled scan triggers.
// CoordinatedRunner ensures both triggers share one process-local scan slot.
type ScanRunner interface {
	Run(ctx context.Context, trigger string) (ScanResult, error)
}

type CoordinatedRunner struct {
	runner ScanRunner
	mu     sync.Mutex
}

func NewCoordinatedRunner(runner ScanRunner) *CoordinatedRunner {
	return &CoordinatedRunner{runner: runner}
}

func (r *CoordinatedRunner) Run(ctx context.Context, trigger string) (ScanResult, error) {
	if r == nil || r.runner == nil {
		return ScanResult{}, errors.New("scan runner is required")
	}
	if !r.mu.TryLock() {
		return ScanResult{}, ErrScanInProgress
	}
	defer r.mu.Unlock()
	return r.runner.Run(ctx, trigger)
}

func (r *CoordinatedRunner) ReprocessPending(ctx context.Context, limit int) (ReprocessResult, error) {
	if r == nil || r.runner == nil {
		return ReprocessResult{}, errors.New("scan runner is required")
	}
	retrier, ok := r.runner.(interface {
		ReprocessPending(context.Context, int) (ReprocessResult, error)
	})
	if !ok {
		return ReprocessResult{}, errors.New("analysis retrier is unavailable")
	}
	return retrier.ReprocessPending(ctx, limit)
}
