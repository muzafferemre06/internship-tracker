package config

import "os"

type Config struct {
	AppEnv               string
	HTTPAddr             string
	AllowedOrigin        string
	DatabasePath         string
	MigrationsPath       string
	CandidateProfilePath string
	SourcesPath          string
	LLMProvider          string
	LLMModel             string
	LLMInputCost         string
	LLMOutputCost        string
	LLMThinkingLevel     string
	OpenRouterAPIKey     string
	GeminiAPIKey         string
	ScanSchedule         string
	ScanTimezone         string
}

func Load() Config {
	return Config{
		AppEnv:         envOrDefault("APP_ENV", "development"),
		HTTPAddr:       envOrDefault("HTTP_ADDR", ":8080"),
		AllowedOrigin:  envOrDefault("ALLOWED_ORIGIN", "http://localhost:5173"),
		DatabasePath:   envOrDefault("DATABASE_PATH", "data/internship-tracker.db"),
		MigrationsPath: envOrDefault("MIGRATIONS_PATH", "migrations"),
		CandidateProfilePath: envOrDefault(
			"CANDIDATE_PROFILE_PATH",
			"configs/candidate-profile.json",
		),
		SourcesPath:      envOrDefault("SOURCES_PATH", "configs/sources.json"),
		LLMProvider:      envOrDefault("LLM_PROVIDER", "deterministic"),
		LLMModel:         os.Getenv("LLM_MODEL"),
		LLMInputCost:     envOrDefault("LLM_INPUT_COST_PER_MILLION_USD", "0"),
		LLMOutputCost:    envOrDefault("LLM_OUTPUT_COST_PER_MILLION_USD", "0"),
		LLMThinkingLevel: envOrDefault("LLM_THINKING_LEVEL", "minimal"),
		OpenRouterAPIKey: os.Getenv("OPENROUTER_API_KEY"),
		GeminiAPIKey:     os.Getenv("GEMINI_API_KEY"),
		ScanSchedule:     envOrDefault("SCAN_SCHEDULE", "0 9 * * 1"),
		ScanTimezone:     envOrDefault("SCAN_TIMEZONE", "Europe/Istanbul"),
	}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
