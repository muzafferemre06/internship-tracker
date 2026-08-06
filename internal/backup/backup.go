// Package backup creates and retains consistent SQLite snapshots.
package backup

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/muzaffer/internship-tracker/internal/database"
)

const filenamePrefix = "internship-tracker-"

// Config is deliberately string-based at the environment boundary so malformed
// production values can fail application startup instead of silently changing
// backup behaviour.
type Config struct {
	Enabled   string
	Directory string
	Time      string
	Timezone  string
	Retention string
}

// Service schedules one daily backup. It is disabled unless explicitly enabled.
type Service struct {
	db        *sql.DB
	enabled   bool
	directory string
	at        time.Duration
	location  *time.Location
	retention int
	logger    *slog.Logger

	startOnce sync.Once
	done      chan struct{}
}

// New validates backup configuration before the HTTP server starts. A disabled
// backup service intentionally does not create its directory or any files.
func New(db *sql.DB, cfg Config, logger *slog.Logger) (*Service, error) {
	enabled, err := strconv.ParseBool(strings.TrimSpace(cfg.Enabled))
	if err != nil {
		return nil, fmt.Errorf("BACKUP_ENABLED must be true or false: %w", err)
	}
	service := &Service{enabled: enabled, done: make(chan struct{})}
	if logger == nil {
		logger = slog.Default()
	}
	service.logger = logger
	if !enabled {
		close(service.done)
		return service, nil
	}
	if db == nil {
		return nil, errors.New("sqlite database is required when backups are enabled")
	}
	directory := strings.TrimSpace(cfg.Directory)
	if directory == "" {
		return nil, errors.New("BACKUP_DIRECTORY is required when backups are enabled")
	}
	at, err := parseDailyTime(cfg.Time)
	if err != nil {
		return nil, err
	}
	location, err := time.LoadLocation(strings.TrimSpace(cfg.Timezone))
	if err != nil {
		return nil, fmt.Errorf("load backup timezone %q: %w", cfg.Timezone, err)
	}
	retention, err := strconv.Atoi(strings.TrimSpace(cfg.Retention))
	if err != nil || retention < 1 {
		return nil, errors.New("BACKUP_RETENTION must be a positive integer")
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("create backup directory: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return nil, fmt.Errorf("secure backup directory: %w", err)
	}

	service.db = db
	service.directory = directory
	service.at = at
	service.location = location
	service.retention = retention
	return service, nil
}

func parseDailyTime(value string) (time.Duration, error) {
	parsed, err := time.Parse("15:04", strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("BACKUP_TIME must be in HH:MM format: %w", err)
	}
	return time.Duration(parsed.Hour())*time.Hour + time.Duration(parsed.Minute())*time.Minute, nil
}

// Start is idempotent. Cancelling ctx stops the timer and cancels an active
// snapshot; Wait joins the scheduler during application shutdown.
func (s *Service) Start(ctx context.Context) {
	if !s.enabled {
		return
	}
	s.startOnce.Do(func() {
		go func() {
			defer close(s.done)
			s.loop(ctx)
		}()
	})
}

func (s *Service) Wait(ctx context.Context) error {
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Service) loop(ctx context.Context) {
	for {
		next := s.next(time.Now().In(s.location))
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
			result, err := s.Run(ctx)
			if err != nil {
				s.logger.Error("scheduled SQLite backup failed", "error", err)
				continue
			}
			s.logger.Info("scheduled SQLite backup completed", "path", result.Path, "created_at", result.CreatedAt)
		}
	}
}

func (s *Service) next(after time.Time) time.Time {
	hour := int(s.at / time.Hour)
	minute := int((s.at % time.Hour) / time.Minute)
	candidate := time.Date(after.Year(), after.Month(), after.Day(), hour, minute, 0, 0, s.location)
	if !candidate.After(after) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return candidate
}

type Result struct {
	Path      string
	CreatedAt time.Time
}

// Run creates a consistent snapshot in a temporary name, validates it, then
// atomically publishes it with owner-only permissions and applies retention.
func (s *Service) Run(ctx context.Context) (Result, error) {
	if !s.enabled {
		return Result{}, errors.New("SQLite backups are disabled")
	}
	createdAt := time.Now().In(s.location)
	filename := filenamePrefix + createdAt.Format("20060102T150405.000000000Z0700") + ".db"
	finalPath := filepath.Join(s.directory, filename)
	temporaryPath := finalPath + ".partial"
	if err := os.Remove(temporaryPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Result{}, fmt.Errorf("remove stale temporary SQLite backup: %w", err)
	}
	defer os.Remove(temporaryPath)
	if err := database.Backup(ctx, s.db, temporaryPath); err != nil {
		return Result{}, err
	}
	if err := os.Chmod(temporaryPath, 0o600); err != nil {
		return Result{}, fmt.Errorf("secure temporary backup: %w", err)
	}
	if err := checkIntegrity(ctx, temporaryPath); err != nil {
		return Result{}, err
	}
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return Result{}, fmt.Errorf("publish SQLite backup: %w", err)
	}
	if err := s.prune(); err != nil {
		return Result{}, err
	}
	return Result{Path: finalPath, CreatedAt: createdAt}, nil
}

func checkIntegrity(ctx context.Context, path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("open SQLite backup for integrity check: %w", err)
	}
	defer db.Close()
	var result string
	if err := db.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&result); err != nil {
		return fmt.Errorf("run SQLite backup integrity check: %w", err)
	}
	if result != "ok" {
		return fmt.Errorf("SQLite backup integrity check failed: %s", result)
	}
	return nil
}

func (s *Service) prune() error {
	entries, err := os.ReadDir(s.directory)
	if err != nil {
		return fmt.Errorf("list SQLite backups: %w", err)
	}
	backups := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), filenamePrefix) || !strings.HasSuffix(entry.Name(), ".db") {
			continue
		}
		backups = append(backups, entry.Name())
	}
	sort.Strings(backups)
	for len(backups) > s.retention {
		path := filepath.Join(s.directory, backups[0])
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove expired SQLite backup %q: %w", path, err)
		}
		backups = backups[1:]
	}
	return nil
}
