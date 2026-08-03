package config

import "testing"

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("DATABASE_PATH", "")

	cfg := Load()

	if cfg.AppEnv != "development" {
		t.Fatalf("expected development, got %q", cfg.AppEnv)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("expected :8080, got %q", cfg.HTTPAddr)
	}
	if cfg.DatabasePath != "data/internship-tracker.db" {
		t.Fatalf("unexpected database path %q", cfg.DatabasePath)
	}
	if cfg.MigrationsPath != "migrations" {
		t.Fatalf("unexpected migrations path %q", cfg.MigrationsPath)
	}
	if cfg.CandidateProfilePath != "configs/candidate-profile.json" {
		t.Fatalf("unexpected candidate profile path %q", cfg.CandidateProfilePath)
	}
	if cfg.LLMProvider != "deterministic" {
		t.Fatalf("unexpected default analyzer %q", cfg.LLMProvider)
	}
	if cfg.LLMThinkingLevel != "minimal" {
		t.Fatalf("unexpected default thinking level %q", cfg.LLMThinkingLevel)
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("LLM_PROVIDER", "openrouter")
	t.Setenv("LLM_MODEL", "example/model")
	t.Setenv("GEMINI_API_KEY", "test-key")

	cfg := Load()

	if cfg.HTTPAddr != ":9090" || cfg.LLMModel != "example/model" || cfg.GeminiAPIKey != "test-key" {
		t.Fatalf("environment was not loaded: %#v", cfg)
	}
}
