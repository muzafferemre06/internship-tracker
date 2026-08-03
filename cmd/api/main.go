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
	"github.com/muzaffer/internship-tracker/internal/config"
	"github.com/muzaffer/internship-tracker/internal/database"
	"github.com/muzaffer/internship-tracker/internal/domain"
	"github.com/muzaffer/internship-tracker/internal/httpapi"
	"github.com/muzaffer/internship-tracker/internal/orchestrator"
	"github.com/muzaffer/internship-tracker/internal/scraper"
	"github.com/muzaffer/internship-tracker/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	db, err := database.Open(context.Background(), cfg.DatabasePath, os.DirFS(cfg.MigrationsPath))
	if err != nil {
		logger.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

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
	sources, err := configureSources(context.Background(), sourcesConfig, repository)
	if err != nil {
		logger.Error("source initialization failed", "error", err)
		os.Exit(1)
	}
	listingAnalyzer, err := configureAnalyzer(cfg)
	if err != nil {
		logger.Error("listing analyzer initialization failed", "error", err)
		os.Exit(1)
	}
	scanService := &orchestrator.Service{
		Sources:  sources,
		Analyzer: listingAnalyzer,
		Store:    repository,
		Profile:  analyzerProfile(candidateConfig),
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           httpapi.NewHandler(cfg.AllowedOrigin, logger, scanService, repository),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Info("api starting", "address", cfg.HTTPAddr, "environment", cfg.AppEnv)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}()

	<-shutdownSignals
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("api stopped")
}

func configureSources(
	ctx context.Context,
	configured config.SourcesConfig,
	repository *store.SQLiteRepository,
) ([]scraper.Source, error) {
	sources := make([]scraper.Source, 0)
	for _, company := range configured.Companies {
		for _, sourceConfig := range company.Sources {
			registration := domain.SourceRegistration{
				Key:           sourceConfig.ID,
				Company:       company.Name,
				PriorityGroup: company.PriorityGroup,
				Type:          sourceConfig.Type,
				URL:           sourceConfig.URL,
				Adapter:       sourceConfig.Adapter,
				Enabled:       sourceConfig.Enabled,
			}
			if err := repository.RegisterSource(ctx, registration); err != nil {
				return nil, err
			}
			if !sourceConfig.Enabled {
				continue
			}

			switch sourceConfig.Adapter {
			case "kariyer_net":
				pageName := sourceConfig.PageName
				if strings.TrimSpace(pageName) == "" {
					pageName = company.Name
				}
				source, err := scraper.NewKariyerNetSource(
					sourceConfig.ID,
					company.Name,
					pageName,
					sourceConfig.URL,
					nil,
				)
				if err != nil {
					return nil, fmt.Errorf("configure source %q: %w", sourceConfig.ID, err)
				}
				sources = append(sources, source)
			default:
				return nil, fmt.Errorf("source %q uses unsupported adapter %q", sourceConfig.ID, sourceConfig.Adapter)
			}
		}
	}
	return sources, nil
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

func configureAnalyzer(cfg config.Config) (analyzer.ListingAnalyzer, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.LLMProvider)) {
	case "deterministic":
		return analyzer.NewDeterministicAnalyzer(), nil
	case "openrouter":
		provider, err := analyzer.NewOpenRouterProvider(cfg.OpenRouterAPIKey, &http.Client{Timeout: 20 * time.Second})
		if err != nil {
			return nil, err
		}
		return newConfiguredModelAnalyzer(provider, cfg)
	case "google", "gemini":
		provider, err := analyzer.NewGoogleProvider(
			cfg.GeminiAPIKey, cfg.LLMThinkingLevel, &http.Client{Timeout: 60 * time.Second},
		)
		if err != nil {
			return nil, err
		}
		return newConfiguredModelAnalyzer(provider, cfg)
	default:
		return nil, fmt.Errorf("unsupported LLM provider %q", cfg.LLMProvider)
	}
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
