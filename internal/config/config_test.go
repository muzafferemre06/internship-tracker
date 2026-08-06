package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadUsesDefaults(t *testing.T) {
	for _, name := range []string{
		"APP_ENV", "HTTP_ADDR", "ALLOWED_ORIGIN", "DATABASE_PATH", "MIGRATIONS_PATH",
		"CANDIDATE_PROFILE_PATH", "SOURCES_PATH", "LLM_PROVIDER", "LLM_MODEL",
		"OPENROUTER_API_KEY", "GEMINI_API_KEY", "SCAN_SCHEDULE", "SCAN_TIMEZONE",
		"WEB_PUSH_ENABLED", "WEB_PUSH_PUBLIC_KEY", "WEB_PUSH_PRIVATE_KEY", "WEB_PUSH_SUBJECT",
		"BACKUP_ENABLED", "BACKUP_DIRECTORY", "BACKUP_TIME", "BACKUP_TIMEZONE", "BACKUP_RETENTION",
	} {
		t.Setenv(name, "")
	}
	unsetSecretFiles(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load defaults: %v", err)
	}
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
	if cfg.ScanSchedule != "0 9 * * 1" || cfg.ScanTimezone != "Europe/Istanbul" {
		t.Fatalf("unexpected default scan schedule settings: %#v", cfg)
	}
	if cfg.WebPushEnabled != "false" || cfg.WebPushPublicKey != "" || cfg.WebPushPrivateKey != "" || cfg.WebPushSubject != "" {
		t.Fatalf("unexpected default Web Push settings: %#v", cfg)
	}
	if cfg.BackupEnabled != "false" || cfg.BackupDirectory != "" || cfg.BackupTime != "02:00" ||
		cfg.BackupTimezone != "Europe/Istanbul" || cfg.BackupRetention != "7" {
		t.Fatalf("unexpected default backup settings: %#v", cfg)
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	unsetSecretFiles(t)
	t.Setenv("APP_ENV", "development")
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("LLM_PROVIDER", "openrouter")
	t.Setenv("LLM_MODEL", "example/model")
	t.Setenv("GEMINI_API_KEY", "test-key")
	t.Setenv("SCAN_SCHEDULE", "30 8 * * 1-5")
	t.Setenv("SCAN_TIMEZONE", "UTC")
	t.Setenv("WEB_PUSH_ENABLED", "true")
	t.Setenv("WEB_PUSH_PUBLIC_KEY", "public")
	t.Setenv("WEB_PUSH_PRIVATE_KEY", "private")
	t.Setenv("WEB_PUSH_SUBJECT", "mailto:push@example.test")
	t.Setenv("BACKUP_ENABLED", "true")
	t.Setenv("BACKUP_DIRECTORY", "/var/lib/tracker-backups")
	t.Setenv("BACKUP_TIME", "03:15")
	t.Setenv("BACKUP_TIMEZONE", "Europe/Athens")
	t.Setenv("BACKUP_RETENTION", "14")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load environment: %v", err)
	}
	if cfg.HTTPAddr != ":9090" || cfg.LLMModel != "example/model" || cfg.GeminiAPIKey != "test-key" {
		t.Fatalf("environment was not loaded: %#v", cfg)
	}
	if cfg.ScanSchedule != "30 8 * * 1-5" || cfg.ScanTimezone != "UTC" {
		t.Fatalf("scan schedule environment was not loaded: %#v", cfg)
	}
	if cfg.BackupEnabled != "true" || cfg.BackupDirectory != "/var/lib/tracker-backups" ||
		cfg.BackupTime != "03:15" || cfg.BackupTimezone != "Europe/Athens" || cfg.BackupRetention != "14" {
		t.Fatalf("backup environment was not loaded: %#v", cfg)
	}
	if cfg.WebPushEnabled != "true" || cfg.WebPushPublicKey != "public" || cfg.WebPushPrivateKey != "private" || cfg.WebPushSubject != "mailto:push@example.test" {
		t.Fatalf("Web Push environment was not loaded: %#v", cfg)
	}
}

func TestLoadReadsSecretsFromFiles(t *testing.T) {
	directory := t.TempDir()
	for _, secret := range []struct{ name, value string }{
		{"OPENROUTER_API_KEY", "openrouter-secret"},
		{"GEMINI_API_KEY", "gemini-secret"},
		{"WEB_PUSH_PRIVATE_KEY", "web-push-secret"},
	} {
		path := filepath.Join(directory, secret.name)
		if err := os.WriteFile(path, []byte("  "+secret.value+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(secret.name, "")
		t.Setenv(secret.name+"_FILE", path)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load file secrets: %v", err)
	}
	if cfg.OpenRouterAPIKey != "openrouter-secret" || cfg.GeminiAPIKey != "gemini-secret" || cfg.WebPushPrivateKey != "web-push-secret" {
		t.Fatalf("file secrets were not loaded")
	}
}

func TestLoadRejectsUnsafeSecretFileConfigurationWithoutLeakingSecret(t *testing.T) {
	for _, test := range []struct {
		name        string
		value       string
		file        string
		wantInError string
	}{
		{name: "both direct and file", value: "must-not-appear", file: "/not-needed", wantInError: "cannot both be set"},
		{name: "empty file path", value: "", file: "", wantInError: "cannot be empty"},
		{name: "unreadable file", value: "", file: "/does/not/exist", wantInError: "read GEMINI_API_KEY_FILE"},
	} {
		t.Run(test.name, func(t *testing.T) {
			unsetSecretFiles(t)
			t.Setenv("GEMINI_API_KEY", test.value)
			t.Setenv("GEMINI_API_KEY_FILE", test.file)
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), test.wantInError) {
				t.Fatalf("expected %q error, got %v", test.wantInError, err)
			}
			if strings.Contains(err.Error(), "must-not-appear") {
				t.Fatalf("secret appeared in error: %v", err)
			}
		})
	}
}

func TestLoadRejectsEmptySecretFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty-secret")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	unsetSecretFiles(t)
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY_FILE", path)
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "must not contain an empty secret") {
		t.Fatalf("expected empty secret error, got %v", err)
	}
}

func TestLoadValidatesProductionRequirements(t *testing.T) {
	t.Run("accepts deterministic analyzer and file private key", func(t *testing.T) {
		setValidProductionEnvironment(t)
		privateKey := filepath.Join(t.TempDir(), "web-push-private-key")
		if err := os.WriteFile(privateKey, []byte("private-key\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("WEB_PUSH_PRIVATE_KEY", "")
		t.Setenv("WEB_PUSH_PRIVATE_KEY_FILE", privateKey)
		cfg, err := Load()
		if err != nil || cfg.LLMProvider != "deterministic" || cfg.WebPushPrivateKey != "private-key" {
			t.Fatalf("valid production config was rejected: cfg=%#v err=%v", cfg, err)
		}
	})

	for _, test := range []struct {
		name, variable, value, want string
	}{
		{"http origin", "ALLOWED_ORIGIN", "http://tracker.example.test", "HTTPS origin"},
		{"localhost origin", "ALLOWED_ORIGIN", "https://localhost", "must not use localhost"},
		{"localhost subdomain origin", "ALLOWED_ORIGIN", "https://app.localhost", "must not use localhost"},
		{"origin path", "ALLOWED_ORIGIN", "https://tracker.example.test/app", "without a path"},
		{"memory database", "DATABASE_PATH", ":memory:", "DATABASE_PATH"},
		{"relative migrations", "MIGRATIONS_PATH", "migrations", "MIGRATIONS_PATH"},
		{"backup disabled", "BACKUP_ENABLED", "false", "BACKUP_ENABLED must be true"},
		{"relative backup directory", "BACKUP_DIRECTORY", "backups", "BACKUP_DIRECTORY"},
		{"push disabled", "WEB_PUSH_ENABLED", "false", "WEB_PUSH_ENABLED must be true"},
		{"missing push private key", "WEB_PUSH_PRIVATE_KEY", "", "WEB_PUSH_PRIVATE_KEY"},
	} {
		t.Run(test.name, func(t *testing.T) {
			setValidProductionEnvironment(t)
			t.Setenv(test.variable, test.value)
			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected %q error, got %v", test.want, err)
			}
		})
	}
}

func setValidProductionEnvironment(t *testing.T) {
	t.Helper()
	unsetSecretFiles(t)
	directory := t.TempDir()
	t.Setenv("APP_ENV", "production")
	t.Setenv("ALLOWED_ORIGIN", "https://tracker.example.test")
	t.Setenv("DATABASE_PATH", filepath.Join(directory, "tracker.db"))
	t.Setenv("MIGRATIONS_PATH", filepath.Join(directory, "migrations"))
	t.Setenv("CANDIDATE_PROFILE_PATH", filepath.Join(directory, "candidate.json"))
	t.Setenv("SOURCES_PATH", filepath.Join(directory, "sources.json"))
	t.Setenv("BACKUP_ENABLED", "true")
	t.Setenv("BACKUP_DIRECTORY", filepath.Join(directory, "backups"))
	t.Setenv("WEB_PUSH_ENABLED", "true")
	t.Setenv("WEB_PUSH_PRIVATE_KEY", "private-key")
	t.Setenv("LLM_PROVIDER", "deterministic")
}

func unsetSecretFiles(t *testing.T) {
	t.Helper()
	for _, name := range []string{"OPENROUTER_API_KEY_FILE", "GEMINI_API_KEY_FILE", "WEB_PUSH_PRIVATE_KEY_FILE"} {
		previous, present := os.LookupEnv(name)
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if present {
				_ = os.Setenv(name, previous)
				return
			}
			_ = os.Unsetenv(name)
		})
	}
}
