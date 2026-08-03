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
}

func TestLoadReadsEnvironment(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("LLM_PROVIDER", "openrouter")
	t.Setenv("LLM_MODEL", "example/model")

	cfg := Load()

	if cfg.HTTPAddr != ":9090" || cfg.LLMModel != "example/model" {
		t.Fatalf("environment was not loaded: %#v", cfg)
	}
}
