// Package scheduler starts in-process, timezone-aware scheduled scans.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/muzaffer/internship-tracker/internal/orchestrator"
)

type Scheduler struct {
	schedule cronExpression
	location *time.Location
	runner   orchestrator.ScanRunner
	logger   *slog.Logger

	startOnce sync.Once
	done      chan struct{}
}

// New validates a five-field cron expression and IANA timezone before the API
// begins listening. Cron fields are minute, hour, day-of-month, month and
// day-of-week; each accepts *, numbers, lists, ranges, and / steps.
func New(expression, timezone string, runner orchestrator.ScanRunner, logger *slog.Logger) (*Scheduler, error) {
	if runner == nil {
		return nil, errors.New("scheduled scan runner is required")
	}
	schedule, err := parseCron(expression)
	if err != nil {
		return nil, err
	}
	location, err := time.LoadLocation(strings.TrimSpace(timezone))
	if err != nil {
		return nil, fmt.Errorf("load scan timezone %q: %w", timezone, err)
	}
	if _, matches := schedule.next(time.Now().In(location)); !matches {
		return nil, fmt.Errorf("invalid scan schedule %q: it has no future calendar match", expression)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{
		schedule: schedule,
		location: location,
		runner:   runner,
		logger:   logger,
		done:     make(chan struct{}),
	}, nil
}

// Start is idempotent. Cancelling ctx stops the timer and cancels an active
// scheduled scan; Wait can then be used during application shutdown.
func (s *Scheduler) Start(ctx context.Context) {
	s.startOnce.Do(func() {
		go func() {
			defer close(s.done)
			s.loop(ctx)
		}()
	})
}

func (s *Scheduler) Wait(ctx context.Context) error {
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Scheduler) loop(ctx context.Context) {
	for {
		next, matches := s.schedule.next(time.Now().In(s.location))
		if !matches {
			s.logger.Error("scheduled scan stopped: schedule has no future calendar match")
			return
		}
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case <-timer.C:
			if ctx.Err() != nil {
				return
			}
			s.run(ctx)
		}
	}
}

func (s *Scheduler) run(ctx context.Context) {
	result, err := s.runner.Run(ctx, "scheduled")
	if err != nil {
		s.logger.Error("scheduled scan failed", "error", err)
		return
	}
	s.logger.Info("scheduled scan completed",
		"run_id", result.RunID,
		"status", result.Status,
		"started_at", result.StartedAt,
		"finished_at", result.FinishedAt,
	)
}

type cronExpression struct {
	minute     cronField
	hour       cronField
	dayOfMonth cronField
	month      cronField
	dayOfWeek  cronField
}

func parseCron(value string) (cronExpression, error) {
	parts := strings.Fields(value)
	if len(parts) != 5 {
		return cronExpression{}, fmt.Errorf("invalid scan schedule %q: expected five cron fields", value)
	}
	minute, err := parseCronField("minute", parts[0], 0, 59, false)
	if err != nil {
		return cronExpression{}, err
	}
	hour, err := parseCronField("hour", parts[1], 0, 23, false)
	if err != nil {
		return cronExpression{}, err
	}
	dayOfMonth, err := parseCronField("day-of-month", parts[2], 1, 31, false)
	if err != nil {
		return cronExpression{}, err
	}
	month, err := parseCronField("month", parts[3], 1, 12, false)
	if err != nil {
		return cronExpression{}, err
	}
	dayOfWeek, err := parseCronField("day-of-week", parts[4], 0, 7, true)
	if err != nil {
		return cronExpression{}, err
	}
	return cronExpression{minute, hour, dayOfMonth, month, dayOfWeek}, nil
}

func (c cronExpression) Next(after time.Time) time.Time {
	next, _ := c.next(after)
	return next
}

func (c cronExpression) next(after time.Time) (time.Time, bool) {
	candidate := after.Truncate(time.Minute).Add(time.Minute)
	const maxMinutes = 8 * 366 * 24 * 60
	for checked := 0; checked < maxMinutes; checked++ {
		if c.matches(candidate) {
			return candidate, true
		}
		candidate = candidate.Add(time.Minute)
	}
	return time.Time{}, false
}

func (c cronExpression) matches(value time.Time) bool {
	if !c.minute.matches(value.Minute()) || !c.hour.matches(value.Hour()) ||
		!c.month.matches(int(value.Month())) {
		return false
	}
	dayOfMonthMatches := c.dayOfMonth.matches(value.Day())
	dayOfWeekMatches := c.dayOfWeek.matches(int(value.Weekday()))
	switch {
	case c.dayOfMonth.any && c.dayOfWeek.any:
		return true
	case c.dayOfMonth.any:
		return dayOfWeekMatches
	case c.dayOfWeek.any:
		return dayOfMonthMatches
	default:
		return dayOfMonthMatches || dayOfWeekMatches
	}
}

type cronField struct {
	values map[int]bool
	any    bool
}

func parseCronField(name, value string, min, max int, sundayAliasesZero bool) (cronField, error) {
	field := cronField{values: make(map[int]bool)}
	for _, element := range strings.Split(value, ",") {
		element = strings.TrimSpace(element)
		if element == "" {
			return cronField{}, fmt.Errorf("invalid cron %s field %q", name, value)
		}
		base, step, err := splitStep(element)
		if err != nil {
			return cronField{}, fmt.Errorf("invalid cron %s field %q: %w", name, value, err)
		}
		start, end := min, max
		if base == "*" {
			if element == "*" {
				field.any = true
			}
		} else if strings.Contains(base, "-") {
			bounds := strings.Split(base, "-")
			if len(bounds) != 2 {
				return cronField{}, fmt.Errorf("invalid cron %s field %q", name, value)
			}
			start, err = parseCronNumber(bounds[0], min, max)
			if err == nil {
				end, err = parseCronNumber(bounds[1], min, max)
			}
			if err != nil || start > end {
				return cronField{}, fmt.Errorf("invalid cron %s field %q", name, value)
			}
		} else {
			start, err = parseCronNumber(base, min, max)
			if err != nil {
				return cronField{}, fmt.Errorf("invalid cron %s field %q", name, value)
			}
			end = start
			if strings.Contains(element, "/") {
				end = max
			}
		}
		for number := start; number <= end; number += step {
			if sundayAliasesZero && number == 7 {
				number = 0
				field.values[number] = true
				break
			}
			field.values[number] = true
		}
	}
	return field, nil
}

func splitStep(value string) (string, int, error) {
	parts := strings.Split(value, "/")
	if len(parts) > 2 || parts[0] == "" {
		return "", 0, errors.New("invalid step")
	}
	if len(parts) == 1 {
		return parts[0], 1, nil
	}
	step, err := strconv.Atoi(parts[1])
	if err != nil || step <= 0 {
		return "", 0, errors.New("step must be a positive integer")
	}
	return parts[0], step, nil
}

func parseCronNumber(value string, min, max int) (int, error) {
	number, err := strconv.Atoi(value)
	if err != nil || number < min || number > max {
		return 0, errors.New("number is outside the allowed range")
	}
	return number, nil
}

func (f cronField) matches(value int) bool {
	return f.values[value]
}
