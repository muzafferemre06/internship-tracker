package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

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
	BackupEnabled        string
	BackupDirectory      string
	BackupTime           string
	BackupTimezone       string
	BackupRetention      string
	WebPushEnabled       string
	WebPushPublicKey     string
	WebPushPrivateKey    string
	WebPushSubject       string
}

// Load reads environment configuration without ever including secret values in
// an error. A secret can be given directly or by a file path, but never both.
func Load() (Config, error) {
	appEnv := envOrDefault("APP_ENV", "development")
	production := strings.EqualFold(strings.TrimSpace(appEnv), "production")

	openRouterAPIKey, err := loadSecret("OPENROUTER_API_KEY", production)
	if err != nil {
		return Config{}, err
	}
	geminiAPIKey, err := loadSecret("GEMINI_API_KEY", production)
	if err != nil {
		return Config{}, err
	}
	webPushPrivateKey, err := loadSecret("WEB_PUSH_PRIVATE_KEY", production)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv:         appEnv,
		HTTPAddr:       envOrDefault("HTTP_ADDR", ":8080"),
		AllowedOrigin:  envOrDefault("ALLOWED_ORIGIN", "http://localhost:5173"),
		DatabasePath:   envOrDefault("DATABASE_PATH", "data/internship-tracker.db"),
		MigrationsPath: envOrDefault("MIGRATIONS_PATH", "migrations"),
		CandidateProfilePath: envOrDefault(
			"CANDIDATE_PROFILE_PATH",
			"configs/candidate-profile.json",
		),
		SourcesPath:       envOrDefault("SOURCES_PATH", "configs/sources.json"),
		LLMProvider:       envOrDefault("LLM_PROVIDER", "deterministic"),
		LLMModel:          os.Getenv("LLM_MODEL"),
		LLMInputCost:      envOrDefault("LLM_INPUT_COST_PER_MILLION_USD", "0"),
		LLMOutputCost:     envOrDefault("LLM_OUTPUT_COST_PER_MILLION_USD", "0"),
		LLMThinkingLevel:  envOrDefault("LLM_THINKING_LEVEL", "minimal"),
		OpenRouterAPIKey:  openRouterAPIKey,
		GeminiAPIKey:      geminiAPIKey,
		ScanSchedule:      envOrDefault("SCAN_SCHEDULE", "0 9 * * 1"),
		ScanTimezone:      envOrDefault("SCAN_TIMEZONE", "Europe/Istanbul"),
		BackupEnabled:     envOrDefault("BACKUP_ENABLED", "false"),
		BackupDirectory:   os.Getenv("BACKUP_DIRECTORY"),
		BackupTime:        envOrDefault("BACKUP_TIME", "02:00"),
		BackupTimezone:    envOrDefault("BACKUP_TIMEZONE", "Europe/Istanbul"),
		BackupRetention:   envOrDefault("BACKUP_RETENTION", "7"),
		WebPushEnabled:    envOrDefault("WEB_PUSH_ENABLED", "false"),
		WebPushPublicKey:  os.Getenv("WEB_PUSH_PUBLIC_KEY"),
		WebPushPrivateKey: webPushPrivateKey,
		WebPushSubject:    os.Getenv("WEB_PUSH_SUBJECT"),
	}
	if production {
		if err := validateProduction(cfg); err != nil {
			return Config{}, err
		}
	}
	return cfg, nil
}

func loadSecret(name string, requireAbsoluteFile bool) (string, error) {
	value := os.Getenv(name)
	file, fileConfigured := os.LookupEnv(name + "_FILE")
	if !fileConfigured {
		return value, nil
	}
	if value != "" {
		return "", fmt.Errorf("%s and %s_FILE cannot both be set", name, name)
	}
	file = strings.TrimSpace(file)
	if file == "" {
		return "", fmt.Errorf("%s_FILE cannot be empty", name)
	}
	if requireAbsoluteFile && !filepath.IsAbs(file) {
		return "", fmt.Errorf("%s_FILE must be an absolute path in production", name)
	}
	contents, err := os.ReadFile(file)
	if err != nil {
		return "", fmt.Errorf("read %s_FILE: %w", name, err)
	}
	secret := strings.TrimSpace(string(contents))
	if secret == "" {
		return "", fmt.Errorf("%s_FILE must not contain an empty secret", name)
	}
	return secret, nil
}

func validateProduction(cfg Config) error {
	if err := validateProductionOrigin(cfg.AllowedOrigin); err != nil {
		return err
	}
	for name, value := range map[string]string{
		"DATABASE_PATH":          cfg.DatabasePath,
		"MIGRATIONS_PATH":        cfg.MigrationsPath,
		"CANDIDATE_PROFILE_PATH": cfg.CandidateProfilePath,
		"SOURCES_PATH":           cfg.SourcesPath,
	} {
		if !filepath.IsAbs(value) {
			return fmt.Errorf("%s must be an absolute path in production", name)
		}
	}
	if databasePathIsEphemeral(cfg.DatabasePath) {
		return fmt.Errorf("DATABASE_PATH must be a persistent filesystem path in production")
	}

	backupEnabled, err := parseBool("BACKUP_ENABLED", cfg.BackupEnabled)
	if err != nil {
		return err
	}
	if !backupEnabled {
		return fmt.Errorf("BACKUP_ENABLED must be true in production")
	}
	if !filepath.IsAbs(cfg.BackupDirectory) {
		return fmt.Errorf("BACKUP_DIRECTORY must be an absolute path in production")
	}

	webPushEnabled, err := parseBool("WEB_PUSH_ENABLED", cfg.WebPushEnabled)
	if err != nil {
		return err
	}
	if !webPushEnabled {
		return fmt.Errorf("WEB_PUSH_ENABLED must be true in production")
	}
	if strings.TrimSpace(cfg.WebPushPrivateKey) == "" {
		return fmt.Errorf("WEB_PUSH_PRIVATE_KEY or WEB_PUSH_PRIVATE_KEY_FILE is required in production")
	}
	return nil
}

func validateProductionOrigin(value string) error {
	origin, err := url.Parse(value)
	if err != nil || origin.Scheme != "https" || origin.Host == "" || origin.User != nil ||
		origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" {
		return fmt.Errorf("ALLOWED_ORIGIN must be an HTTPS origin without a path in production")
	}
	host := strings.TrimSuffix(strings.ToLower(origin.Hostname()), ".")
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("ALLOWED_ORIGIN must not use localhost in production")
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return fmt.Errorf("ALLOWED_ORIGIN must not use a loopback address in production")
	}
	return nil
}

func databasePathIsEphemeral(value string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	return trimmed == ":memory:" || strings.HasPrefix(trimmed, "file:")
}

func parseBool(name, value string) (bool, error) {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", name)
	}
	return parsed, nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
