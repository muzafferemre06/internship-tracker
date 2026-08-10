package scraper

import (
	"context"
	"fmt"
	"time"

	"github.com/muzaffer/internship-tracker/internal/domain"
)

type Source interface {
	Name() string
	FetchListings(ctx context.Context) ([]domain.RawListing, error)
}

type AccessPolicy struct {
	Mode            string
	Scope           string
	TargetURL       string
	MinimumInterval time.Duration
	BaseCooldown    time.Duration
	MaximumCooldown time.Duration
}

type ProtectedSource interface {
	Source
	AccessPolicy() AccessPolicy
}

type RobotsDecision struct {
	Allowed bool
	Reason  string
}

type RobotsChecker interface {
	Check(ctx context.Context, policy AccessPolicy) (RobotsDecision, error)
}

type AccessError struct {
	StatusCode int
	RetryAfter *time.Time
	Server     string
	CFRay      string
	Challenge  bool
}

func (e *AccessError) Error() string {
	if e.Challenge && e.StatusCode == 0 {
		return "access protection challenge detected"
	}
	if e.Challenge {
		return fmt.Sprintf("access protection challenge returned HTTP status %d", e.StatusCode)
	}
	return fmt.Sprintf("unexpected HTTP status %d", e.StatusCode)
}

func (e *AccessError) Protective() bool {
	return e.Challenge || e.StatusCode == 403 || e.StatusCode == 429
}
