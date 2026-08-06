package push

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/store"
)

type fakeDeliveryRepository struct {
	deliveries []store.PushDelivery
	sent       []int64
	retried    []int64
	failed     []int64
	disabled   []int64
}

func (f *fakeDeliveryRepository) ClaimPushDeliveries(context.Context, int, time.Time, time.Duration) ([]store.PushDelivery, error) {
	return f.deliveries, nil
}
func (f *fakeDeliveryRepository) MarkPushDeliverySent(_ context.Context, id int64, _ time.Time, _ int) error {
	f.sent = append(f.sent, id)
	return nil
}
func (f *fakeDeliveryRepository) RetryPushDelivery(_ context.Context, id int64, _ time.Time, _ int, _ string) error {
	f.retried = append(f.retried, id)
	return nil
}
func (f *fakeDeliveryRepository) FailPushDelivery(_ context.Context, id int64, _ time.Time, _ int, _ string) error {
	f.failed = append(f.failed, id)
	return nil
}
func (f *fakeDeliveryRepository) DisablePushSubscription(_ context.Context, id int64, _ time.Time, _ int) error {
	f.disabled = append(f.disabled, id)
	return nil
}

type sequenceSender struct {
	results []SendResult
	errors  []error
	calls   int
}

func (s *sequenceSender) Send(context.Context, store.PushSubscription, Message) (SendResult, error) {
	index := s.calls
	s.calls++
	return s.results[index], s.errors[index]
}

func TestDispatcherHandlesMultiDeviceRetryAndGone(t *testing.T) {
	repository := &fakeDeliveryRepository{deliveries: []store.PushDelivery{
		{ID: 1, AttemptCount: 1, Title: "one", Body: "body", TargetURL: "/?listing=1", Topic: "one"},
		{ID: 2, AttemptCount: 2, Title: "two", Body: "body", TargetURL: "/?listing=2", Topic: "two"},
		{ID: 3, AttemptCount: 1, Title: "three", Body: "body", TargetURL: "/?listing=3", Topic: "three"},
	}}
	sender := &sequenceSender{
		results: []SendResult{{StatusCode: 201}, {StatusCode: 429}, {StatusCode: 410}},
		errors:  []error{nil, nil, nil},
	}
	dispatcher, err := NewDispatcher(repository, sender, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if err := dispatcher.DispatchPending(context.Background()); err != nil {
		t.Fatalf("dispatch pending: %v", err)
	}
	if len(repository.sent) != 1 || repository.sent[0] != 1 || len(repository.retried) != 1 || repository.retried[0] != 2 ||
		len(repository.disabled) != 1 || repository.disabled[0] != 3 {
		t.Fatalf("unexpected outcomes: sent=%v retried=%v disabled=%v", repository.sent, repository.retried, repository.disabled)
	}
}

func TestDispatcherStopsRetryingAtAttemptLimit(t *testing.T) {
	repository := &fakeDeliveryRepository{deliveries: []store.PushDelivery{{
		ID: 9, AttemptCount: 5, Title: "title", Body: "body", TargetURL: "/", Topic: "topic",
	}}}
	sender := &sequenceSender{results: []SendResult{{}}, errors: []error{errors.New("timeout")}}
	dispatcher, _ := NewDispatcher(repository, sender, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := dispatcher.DispatchPending(context.Background()); err != nil {
		t.Fatalf("persisted terminal delivery should not fail the batch: %v", err)
	}
	if len(repository.failed) != 1 || len(repository.retried) != 0 {
		t.Fatalf("unexpected retry outcome: failed=%v retried=%v", repository.failed, repository.retried)
	}
}
