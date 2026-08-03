package config

import "os"

type Config struct {
	AppEnv          string
	HTTPAddr        string
	AllowedOrigin   string
	DatabasePath    string
	LLMProvider     string
	LLMModel        string
	OpenRouterAPIKey string
	ScanSchedule    string
}

func Load() Config {
	return Config{
		AppEnv:           envOrDefault("APP_ENV", "development"),
		HTTPAddr:         envOrDefault("HTTP_ADDR", ":8080"),
		AllowedOrigin:    envOrDefault("ALLOWED_ORIGIN", "http://localhost:5173"),
		DatabasePath:     envOrDefault("DATABASE_PATH", "data/internship-tracker.db"),
		LLMProvider:      envOrDefault("LLM_PROVIDER", "openrouter"),
		LLMModel:         os.Getenv("LLM_MODEL"),
		OpenRouterAPIKey: os.Getenv("OPENROUTER_API_KEY"),
		ScanSchedule:     envOrDefault("SCAN_SCHEDULE", "0 9 * * 1"),
	}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
