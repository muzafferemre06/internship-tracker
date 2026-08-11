package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/muzaffer/internship-tracker/internal/analyzer"
	"github.com/muzaffer/internship-tracker/internal/backup"
	"github.com/muzaffer/internship-tracker/internal/config"
	"github.com/muzaffer/internship-tracker/internal/database"
	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/extractor"
	"github.com/muzaffer/internship-tracker/internal/httpapi"
	"github.com/muzaffer/internship-tracker/internal/orchestrator"
	"github.com/muzaffer/internship-tracker/internal/push"
	"github.com/muzaffer/internship-tracker/internal/scheduler"
	"github.com/muzaffer/internship-tracker/internal/scraper"
	"github.com/muzaffer/internship-tracker/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration initialization failed", "error", err)
		os.Exit(1)
	}

	db, err := database.Open(context.Background(), cfg.DatabasePath, os.DirFS(cfg.MigrationsPath))
	if err != nil {
		logger.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()
	readinessChecker, err := database.NewReadinessChecker(db, os.DirFS(cfg.MigrationsPath))
	if err != nil {
		logger.Error("database readiness initialization failed", "error", err)
		os.Exit(1)
	}

	candidateConfig, err := config.LoadCandidateProfile(cfg.CandidateProfilePath)
	if err != nil {
		logger.Error("candidate profile initialization failed", "error", err)
		os.Exit(1)
	}
	sourcesConfig, err := config.LoadSources(cfg.SourcesPath)
	if err != nil {
		logger.Error("source configuration initialization failed", "error", err)
		os.Exit(1)
	}

	repository, err := store.NewSQLiteRepository(db)
	if err != nil {
		logger.Error("repository initialization failed", "error", err)
		os.Exit(1)
	}
	if err := repository.ReconcileOpportunities(context.Background()); err != nil {
		logger.Error("canonical opportunity reconciliation failed", "error", err)
		os.Exit(1)
	}
	modelProvider, err := configureModelProvider(cfg)
	if err != nil {
		logger.Error("model provider initialization failed", "error", err)
		os.Exit(1)
	}
	listingExtractor, err := configureExtractorFromProvider(modelProvider, cfg)
	if err != nil {
		logger.Error("listing extractor initialization failed", "error", err)
		os.Exit(1)
	}
	recipeLearner, err := configureRecipeLearnerFromProvider(modelProvider, cfg)
	if err != nil {
		logger.Error("recipe learner initialization failed", "error", err)
		os.Exit(1)
	}
	sources, err := configureSources(context.Background(), sourcesConfig, repository, scraper.SourceDeps{
		Extractor: listingExtractor, RecipeStore: repository, RecipeLearner: recipeLearner, BlockCache: repository,
	})
	if err != nil {
		logger.Error("source initialization failed", "error", err)
		os.Exit(1)
	}
	listingAnalyzer, err := configureAnalyzerFromProvider(modelProvider, cfg)
	if err != nil {
		logger.Error("listing analyzer initialization failed", "error", err)
		os.Exit(1)
	}
	scanService := &orchestrator.Service{
		Sources:  sources,
		Analyzer: listingAnalyzer,
		Store:    repository,
		Robots:   scraper.NewHTTPRobotsChecker(nil, nil),
		Profile:  analyzerProfile(candidateConfig),
	}
	scanRunner := orchestrator.NewCoordinatedRunner(scanService)
	scanScheduler, err := scheduler.New(cfg.ScanSchedule, cfg.ScanTimezone, scanRunner, logger)
	if err != nil {
		logger.Error("scan scheduler initialization failed", "error", err)
		os.Exit(1)
	}
	backupService, err := backup.New(db, backup.Config{
		Enabled: cfg.BackupEnabled, Directory: cfg.BackupDirectory, Time: cfg.BackupTime,
		Timezone: cfg.BackupTimezone, Retention: cfg.BackupRetention,
	}, logger)
	if err != nil {
		logger.Error("SQLite backup initialization failed", "error", err)
		os.Exit(1)
	}
	webPushConfig, err := configureWebPush(cfg)
	if err != nil {
		logger.Error("Web Push configuration failed", "error", err)
		os.Exit(1)
	}
	var pushDispatcher *push.Dispatcher
	if webPushConfig.Enabled {
		pushSender, err := push.NewHTTPSender(webPushConfig, nil)
		if err != nil {
			logger.Error("Web Push sender initialization failed", "error", err)
			os.Exit(1)
		}
		pushDispatcher, err = push.NewDispatcher(repository, pushSender, logger)
		if err != nil {
			logger.Error("Web Push dispatcher initialization failed", "error", err)
			os.Exit(1)
		}
	}

	server := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: httpapi.NewHandler(cfg.AllowedOrigin, logger, scanRunner, repository, readinessChecker, httpapi.Options{
			RequireExactOrigin: strings.EqualFold(strings.TrimSpace(cfg.AppEnv), "production"),
			Push: httpapi.PushOptions{
				Enabled: webPushConfig.Enabled, PublicKey: webPushConfig.PublicKey, Store: repository,
			},
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	appContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	scanScheduler.Start(appContext)
	backupService.Start(appContext)
	if pushDispatcher != nil {
		pushDispatcher.Start(appContext)
	}

	go func() {
		logger.Info("api starting", "address", cfg.HTTPAddr, "environment", cfg.AppEnv,
			"database_path", cfg.DatabasePath,
			"scan_schedule", cfg.ScanSchedule, "scan_timezone", cfg.ScanTimezone,
			"backup_enabled", cfg.BackupEnabled, "backup_time", cfg.BackupTime,
			"backup_timezone", cfg.BackupTimezone, "backup_retention", cfg.BackupRetention,
			"web_push_enabled", webPushConfig.Enabled)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}()

	<-appContext.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	if err := scanScheduler.Wait(ctx); err != nil {
		logger.Error("scheduled scan shutdown failed", "error", err)
		os.Exit(1)
	}
	if err := backupService.Wait(ctx); err != nil {
		logger.Error("SQLite backup shutdown failed", "error", err)
		os.Exit(1)
	}
	if pushDispatcher != nil {
		if err := pushDispatcher.Wait(ctx); err != nil {
			logger.Error("Web Push dispatcher shutdown failed", "error", err)
			os.Exit(1)
		}
	}

	logger.Info("api stopped")
}

func configureWebPush(cfg config.Config) (push.Config, error) {
	enabled, err := strconv.ParseBool(strings.TrimSpace(cfg.WebPushEnabled))
	if err != nil {
		return push.Config{}, fmt.Errorf("WEB_PUSH_ENABLED must be true or false")
	}
	return push.ValidateConfig(push.Config{
		Enabled: enabled, PublicKey: cfg.WebPushPublicKey,
		PrivateKey: cfg.WebPushPrivateKey, Subject: cfg.WebPushSubject,
	})
}

func configureSources(
	ctx context.Context,
	configured config.SourcesConfig,
	repository *store.SQLiteRepository,
	deps scraper.SourceDeps,
) ([]scraper.Source, error) {
	sources := make([]scraper.Source, 0)
	for _, company := range configured.Companies {
		for _, sourceConfig := range company.Sources {
			var runtimePolicy *scraper.AccessPolicy
			registration := domain.SourceRegistration{
				Key:                sourceConfig.ID,
				Company:            company.Name,
				PriorityGroup:      company.PriorityGroup,
				Type:               sourceConfig.Type,
				URL:                sourceConfig.URL,
				Adapter:            sourceConfig.Adapter,
				Strategy:           sourceConfig.EffectiveStrategy(),
				TrackingStatus:     company.EffectiveTrackingStatus(),
				TrackingPhase:      company.TrackingPhase,
				Enabled:            sourceConfig.Enabled,
				CoverageStatus:     sourceConfig.EffectiveCoverageStatus(),
				CoverageReason:     sourceConfig.CoverageReason,
				CoverageReasonCode: sourceConfig.CoverageReasonCode,
				TrustLevel:         sourceConfig.EffectiveTrustLevel(),
			}
			registration.LastVerifiedAt, _ = parseOptionalConfigTime(sourceConfig.LastVerifiedAt)
			if policy, found := configured.ResolveAccessPolicy(sourceConfig.URL); found {
				registration.AccessMode = policy.Mode
				registration.AccessScope = policy.Domain
				registration.MinimumInterval = time.Duration(policy.MinimumIntervalSeconds) * time.Second
				registration.BaseCooldown = time.Duration(policy.BaseCooldownSeconds) * time.Second
				registration.MaximumCooldown = time.Duration(policy.MaximumCooldownSeconds) * time.Second
				resolved := scraper.AccessPolicy{
					Mode: policy.Mode, Scope: policy.Domain, TargetURL: sourceConfig.URL,
					MinimumInterval: registration.MinimumInterval,
					BaseCooldown:    registration.BaseCooldown, MaximumCooldown: registration.MaximumCooldown,
				}
				runtimePolicy = &resolved
			}
			if err := repository.RegisterSource(ctx, registration); err != nil {
				return nil, err
			}
			if !sourceConfig.Enabled {
				continue
			}

			if !scraper.SupportsAdapter(sourceConfig.Adapter) {
				return nil, fmt.Errorf("source %q uses unsupported adapter %q", sourceConfig.ID, sourceConfig.Adapter)
			}
			source, err := scraper.NewSource(sourceConfig.Adapter, scraper.SourceSpec{
				ID:                  sourceConfig.ID,
				Company:             company.Name,
				PageName:            sourceConfig.PageName,
				URL:                 sourceConfig.URL,
				ListingContainerID:  sourceConfig.ListingContainerID,
				ListingPathPrefix:   sourceConfig.ListingPathPrefix,
				ListingAllowedHosts: sourceConfig.ListingAllowedHosts,
				AccessPolicy:        runtimePolicy,
			}, deps)
			if err != nil {
				return nil, fmt.Errorf("configure source %q: %w", sourceConfig.ID, err)
			}
			sources = append(sources, source)
		}
		for _, programConfig := range company.Programs {
			program := domain.ProgramWindow{
				Key: programConfig.ID, Company: company.Name, Name: programConfig.Name,
				Type: programConfig.Type, URL: programConfig.URL, Status: programConfig.Status,
			}
			program.OpensAt, _ = parseOptionalConfigTime(programConfig.OpensAt)
			program.ClosesAt, _ = parseOptionalConfigTime(programConfig.ClosesAt)
			program.LastVerifiedAt, _ = parseOptionalConfigTime(programConfig.LastVerifiedAt)
			if err := repository.RegisterProgramWindow(ctx, program); err != nil {
				return nil, err
			}
		}
	}
	return sources, nil
}

func parseOptionalConfigTime(value string) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func analyzerProfile(profile config.CandidateProfile) analyzer.CandidateProfile {
	experienceAreas := make([]string, 0)
	for _, experience := range profile.Experience {
		experienceAreas = append(experienceAreas, experience.Areas...)
	}
	return analyzer.CandidateProfile{
		EducationField:  strings.TrimSpace(profile.Education.Department),
		ClassYear:       profile.Education.ClassYear,
		GPA:             profile.Education.GPA,
		FocusAreas:      append([]string(nil), profile.FocusAreas...),
		ExperienceAreas: experienceAreas,
		Locations:       append([]string(nil), profile.LocationPreferences.Primary...),
	}
}

// configureModelProvider builds the configured LLM provider, or returns a nil
// provider when the deterministic analyzer is selected (no live model).
func configureModelProvider(cfg config.Config) (analyzer.ModelProvider, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.LLMProvider)) {
	case "deterministic":
		return nil, nil
	case "openrouter":
		return analyzer.NewOpenRouterProvider(cfg.OpenRouterAPIKey, &http.Client{Timeout: 20 * time.Second})
	case "google", "gemini":
		return analyzer.NewGoogleProvider(
			cfg.GeminiAPIKey, cfg.LLMThinkingLevel, &http.Client{Timeout: 60 * time.Second},
		)
	default:
		return nil, fmt.Errorf("unsupported LLM provider %q", cfg.LLMProvider)
	}
}

func configureAnalyzer(cfg config.Config) (analyzer.ListingAnalyzer, error) {
	provider, err := configureModelProvider(cfg)
	if err != nil {
		return nil, err
	}
	return configureAnalyzerFromProvider(provider, cfg)
}

func configureAnalyzerFromProvider(provider analyzer.ModelProvider, cfg config.Config) (analyzer.ListingAnalyzer, error) {
	if provider == nil {
		return analyzer.NewDeterministicAnalyzer(), nil
	}
	return newConfiguredModelAnalyzer(provider, cfg)
}

// configureExtractor builds the Faz 11 listing extractor from the configured
// provider. It returns a nil extractor under the deterministic analyzer; any
// enabled llm_generic source then fails fast with a clear message.
func configureExtractor(cfg config.Config) (scraper.ListingExtractor, error) {
	provider, err := configureModelProvider(cfg)
	if err != nil {
		return nil, err
	}
	return configureExtractorFromProvider(provider, cfg)
}

func configureExtractorFromProvider(provider analyzer.ModelProvider, cfg config.Config) (scraper.ListingExtractor, error) {
	if provider == nil {
		return nil, nil
	}
	return extractor.NewGeminiExtractor(provider, cfg.LLMModel)
}

func configureRecipeLearner(cfg config.Config) (scraper.RecipeLearner, error) {
	provider, err := configureModelProvider(cfg)
	if err != nil {
		return nil, err
	}
	return configureRecipeLearnerFromProvider(provider, cfg)
}

func configureRecipeLearnerFromProvider(provider analyzer.ModelProvider, cfg config.Config) (scraper.RecipeLearner, error) {
	if provider == nil {
		return nil, nil
	}
	return extractor.NewGeminiRecipeLearner(provider, cfg.LLMModel)
}

func newConfiguredModelAnalyzer(provider analyzer.ModelProvider, cfg config.Config) (analyzer.ListingAnalyzer, error) {
	inputCost, err := strconv.ParseFloat(cfg.LLMInputCost, 64)
	if err != nil || inputCost < 0 {
		return nil, fmt.Errorf("LLM_INPUT_COST_PER_MILLION_USD must be a non-negative number")
	}
	outputCost, err := strconv.ParseFloat(cfg.LLMOutputCost, 64)
	if err != nil || outputCost < 0 {
		return nil, fmt.Errorf("LLM_OUTPUT_COST_PER_MILLION_USD must be a non-negative number")
	}
	return analyzer.NewModelAnalyzer(provider, cfg.LLMModel, analyzer.CostRates{
		InputPerMillionUSD: inputCost, OutputPerMillionUSD: outputCost,
	})
}
