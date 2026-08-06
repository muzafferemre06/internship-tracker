package push

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/muzaffer/internship-tracker/internal/store"
)

type Dispatcher struct {
	repository store.PushDeliveryRepository
	sender     Sender
	logger     *slog.Logger
	now        func() time.Time
	interval   time.Duration
	lease      time.Duration
	maxAttempt int

	waitGroup sync.WaitGroup
}

func NewDispatcher(repository store.PushDeliveryRepository, sender Sender, logger *slog.Logger) (*Dispatcher, error) {
	if repository == nil || sender == nil {
		return nil, errors.New("push repository and sender are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Dispatcher{
		repository: repository, sender: sender, logger: logger, now: time.Now,
		interval: 5 * time.Second, lease: 45 * time.Second, maxAttempt: 5,
	}, nil
}

func (d *Dispatcher) Start(ctx context.Context) {
	d.waitGroup.Add(1)
	go func() {
		defer d.waitGroup.Done()
		d.run(ctx)
	}()
}

func (d *Dispatcher) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		d.waitGroup.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (d *Dispatcher) run(ctx context.Context) {
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for {
		if err := d.DispatchPending(ctx); err != nil && !errors.Is(err, context.Canceled) {
			d.logger.Error("push dispatch failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (d *Dispatcher) DispatchPending(ctx context.Context) error {
	deliveries, err := d.repository.ClaimPushDeliveries(ctx, 25, d.now().UTC(), d.lease)
	if err != nil {
		return err
	}
	var dispatchErrors []error
	for _, delivery := range deliveries {
		if err := d.dispatch(ctx, delivery); err != nil {
			dispatchErrors = append(dispatchErrors, err)
		}
	}
	return errors.Join(dispatchErrors...)
}

func (d *Dispatcher) dispatch(ctx context.Context, delivery store.PushDelivery) error {
	if delivery.Subscription.ExpirationAt != nil && !delivery.Subscription.ExpirationAt.After(d.now().UTC()) {
		return d.repository.DisablePushSubscription(context.WithoutCancel(ctx), delivery.ID, d.now().UTC(), http.StatusGone)
	}
	payload, err := json.Marshal(map[string]string{
		"title": delivery.Title,
		"body":  delivery.Body,
		"url":   delivery.TargetURL,
		"tag":   delivery.Topic,
	})
	if err != nil {
		return d.fail(ctx, delivery, 0, err)
	}
	result, sendErr := d.sender.Send(ctx, delivery.Subscription, Message{Payload: payload, Topic: delivery.Topic, TTL: 86400})
	if sendErr == nil && result.StatusCode >= 200 && result.StatusCode < 300 {
		return d.repository.MarkPushDeliverySent(context.WithoutCancel(ctx), delivery.ID, d.now().UTC(), result.StatusCode)
	}
	if result.StatusCode == http.StatusNotFound || result.StatusCode == http.StatusGone {
		return d.repository.DisablePushSubscription(context.WithoutCancel(ctx), delivery.ID, d.now().UTC(), result.StatusCode)
	}
	if transientPushFailure(result.StatusCode, sendErr) && delivery.AttemptCount < d.maxAttempt {
		now := d.now().UTC()
		nextAttempt := now.Add(retryDelay(delivery.AttemptCount))
		if result.RetryAfter != nil && result.RetryAfter.After(nextAttempt) {
			nextAttempt = result.RetryAfter.UTC()
		}
		if maximum := now.Add(24 * time.Hour); nextAttempt.After(maximum) {
			nextAttempt = maximum
		}
		return d.repository.RetryPushDelivery(
			context.WithoutCancel(ctx), delivery.ID, nextAttempt, result.StatusCode, pushFailureReason(sendErr, result.StatusCode),
		)
	}
	return d.fail(ctx, delivery, result.StatusCode, errors.New(pushFailureReason(sendErr, result.StatusCode)))
}

func (d *Dispatcher) fail(ctx context.Context, delivery store.PushDelivery, statusCode int, err error) error {
	if saveErr := d.repository.FailPushDelivery(
		context.WithoutCancel(ctx), delivery.ID, d.now().UTC(), statusCode, err.Error(),
	); saveErr != nil {
		return errors.Join(err, saveErr)
	}
	return nil
}

func transientPushFailure(statusCode int, err error) bool {
	return err != nil || statusCode == http.StatusRequestTimeout || statusCode == http.StatusTooEarly ||
		statusCode == http.StatusTooManyRequests || statusCode >= 500
}

func retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := time.Minute << min(attempt-1, 6)
	if delay > time.Hour {
		return time.Hour
	}
	return delay
}

func pushFailureReason(err error, statusCode int) string {
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("push service returned HTTP %d", statusCode)
}
