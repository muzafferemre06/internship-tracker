package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/muzaffer/internship-tracker/internal/config"
	"github.com/muzaffer/internship-tracker/internal/database"
	"github.com/muzaffer/internship-tracker/internal/scraper"
	"github.com/muzaffer/internship-tracker/internal/store"
)

func TestConfigureAnalyzerSelectsProviderAndValidatesOpenRouterSettings(t *testing.T) {
	deterministic, err := configureAnalyzer(config.Config{LLMProvider: "deterministic"})
	if err != nil || deterministic == nil {
		t.Fatalf("configure deterministic analyzer: analyzer=%#v err=%v", deterministic, err)
	}

	_, err = configureAnalyzer(config.Config{LLMProvider: "openrouter", LLMModel: "model"})
	if err == nil {
		t.Fatal("OpenRouter must require an API key")
	}

	configured, err := configureAnalyzer(config.Config{
		LLMProvider: "openrouter", LLMModel: "provider/model", OpenRouterAPIKey: "secret",
		LLMInputCost: "0.1", LLMOutputCost: "0.2",
	})
	if err != nil || configured == nil {
		t.Fatalf("configure OpenRouter analyzer: analyzer=%#v err=%v", configured, err)
	}

	_, err = configureAnalyzer(config.Config{
		LLMProvider: "google", LLMModel: "gemini-3.1-flash-lite",
		LLMInputCost: "0", LLMOutputCost: "0", LLMThinkingLevel: "minimal",
	})
	if err == nil {
		t.Fatal("Google provider must require a Gemini API key")
	}

	configured, err = configureAnalyzer(config.Config{
		LLMProvider: "google", LLMModel: "gemini-3.1-flash-lite", GeminiAPIKey: "secret",
		LLMInputCost: "0", LLMOutputCost: "0", LLMThinkingLevel: "minimal",
	})
	if err != nil || configured == nil {
		t.Fatalf("configure Google analyzer: analyzer=%#v err=%v", configured, err)
	}
}

func TestConfigureSourcesAppliesResolvedRuntimeAccessPolicy(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "tracker.db"), os.DirFS("../../migrations"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository, err := store.NewSQLiteRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	configured := config.SourcesConfig{
		AccessPolicies: []config.DomainAccessPolicy{{
			Domain: "careers.example.test", Mode: "robots",
			MinimumIntervalSeconds: 2, BaseCooldownSeconds: 60, MaximumCooldownSeconds: 3600,
		}},
		Companies: []config.CompanyConfig{{
			Name: "Example", PriorityGroup: "primary",
			Sources: []config.SourceConfig{{
				ID: "example-careers", Type: "career_page", URL: "https://careers.example.test/jobs",
				Adapter: "json_ld", Strategy: "structured_data", Enabled: true,
			}},
		}},
	}
	sources, err := configureSources(context.Background(), configured, repository, scraper.SourceDeps{})
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 {
		t.Fatalf("configured sources=%d, want 1", len(sources))
	}
	protected, ok := sources[0].(scraper.ProtectedSource)
	if !ok {
		t.Fatalf("configured source does not expose access policy: %T", sources[0])
	}
	want := scraper.AccessPolicy{
		Mode: "robots", Scope: "careers.example.test", TargetURL: "https://careers.example.test/jobs",
		MinimumInterval: 2 * time.Second, BaseCooldown: time.Minute, MaximumCooldown: time.Hour,
	}
	if got := protected.AccessPolicy(); !reflect.DeepEqual(got, want) {
		t.Fatalf("runtime policy=%#v, want %#v", got, want)
	}
}

func TestConfigureSourcesRegistersManualOnlySocialSourceWithoutBuildingScraper(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "tracker.db"), os.DirFS("../../migrations"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	repository, err := store.NewSQLiteRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	configured := config.SourcesConfig{
		AccessPolicies: []config.DomainAccessPolicy{{Domain: "linkedin.com", Mode: "manual_only"}},
		Companies: []config.CompanyConfig{{
			Name: "Havelsan", PriorityGroup: "primary", TrackingStatus: "manual",
			Sources: []config.SourceConfig{{
				ID: "havelsan-linkedin", Type: "social_profile",
				URL:     "https://www.linkedin.com/company/havelsan/jobs/",
				Adapter: "manual", Strategy: "manual", Enabled: false,
			}},
		}},
	}
	sources, err := configureSources(context.Background(), configured, repository, scraper.SourceDeps{})
	if err != nil {
		t.Fatalf("configure manual-only source: %v", err)
	}
	if len(sources) != 0 {
		t.Fatalf("manual-only social source must not build a scraper: %#v", sources)
	}
	var mode string
	if err := db.QueryRow("SELECT access_mode FROM company_sources WHERE source_key = ?", "havelsan-linkedin").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "manual_only" {
		t.Fatalf("manual-only policy was not registered: %q", mode)
	}
}

func TestConfigureRecipeLearnerUsesConfiguredModelProvider(t *testing.T) {
	if learner, err := configureRecipeLearner(config.Config{LLMProvider: "deterministic"}); err != nil || learner != nil {
		t.Fatalf("deterministic mode must not create recipe learner: learner=%#v err=%v", learner, err)
	}
	learner, err := configureRecipeLearner(config.Config{
		LLMProvider: "google", LLMModel: "gemini-3.1-flash-lite", GeminiAPIKey: "secret", LLMThinkingLevel: "minimal",
	})
	if err != nil || learner == nil {
		t.Fatalf("Google mode must create recipe learner: learner=%#v err=%v", learner, err)
	}
}

func TestAnalyzerProfileRemovesInstitutionNames(t *testing.T) {
	profile := analyzerProfile(config.CandidateProfile{
		Education:  config.EducationProfile{University: "Private University", Department: "CTIS", ClassYear: 2},
		Experience: []config.ExperienceProfile{{Organization: "Private Company", Areas: []string{"backend"}}},
	})
	if profile.EducationField != "CTIS" || len(profile.ExperienceAreas) != 1 || profile.ExperienceAreas[0] != "backend" {
		t.Fatalf("required profile attributes were not mapped: %#v", profile)
	}
}
