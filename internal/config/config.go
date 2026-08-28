// Package config loads runtime configuration from environment variables. Kept as a single flat
// struct read once at startup — no config library, no reflection-based env binding.
package config

import "os"

type Config struct {
	Port            string
	DBPath          string
	AuthSecret      string
	AppURL          string
	MediaStorageDir string

	BootstrapAdminName     string
	BootstrapAdminEmail    string
	BootstrapAdminPassword string

	// SeedExampleData additionally seeds fictional demo content (clients, assets, a project, a
	// treatment, a report, media library images, and one non-admin example user) — see
	// internal/seed/demo.go. Never true for a production deploy (see
	// ansible/roles/studio_app/defaults/main.yml); local .env and docker-compose.yml both default
	// it to true.
	SeedExampleData bool

	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string
	SMTPFrom string

	// LogLevel is one of "debug"/"info"/"warn"/"error" (case-insensitive), defaulting to "info" -
	// see internal/logging. Every request still gets its one access-log line regardless of level
	// (at info, or warn/error for a 4xx/5xx response) - "debug" only adds extra low-level detail
	// individual packages may choose to log at that level.
	LogLevel string
	// LogFormat is "text" (human-readable key=value, easy to read directly in `journalctl -f`) or
	// "json" (machine-parseable, pipe through `jq`) - defaults to "text". Only affects the console
	// (stdout/journalctl) stream - the on-disk file under LogDir, if any, is always JSON.
	LogFormat string
	// LogDir is where a rotation-friendly JSON log file (LogDir/studio.log) is written, in
	// addition to stdout - blank disables file logging entirely (console-only). See
	// internal/logging and docs/manage.md for the logrotate setup that keeps it capped at 10MB.
	LogDir string
}

func Load() *Config {
	return &Config{
		Port:            getenv("PORT", "3000"),
		DBPath:          getenv("DB_PATH", "./data/studio.db"),
		AuthSecret:      getenv("AUTH_SECRET", "dev-only-insecure-secret-change-me"),
		AppURL:          os.Getenv("APP_URL"),
		MediaStorageDir: getenv("MEDIA_STORAGE_DIR", "./data/media-storage"),

		BootstrapAdminName:     os.Getenv("BOOTSTRAP_ADMIN_NAME"),
		BootstrapAdminEmail:    os.Getenv("BOOTSTRAP_ADMIN_EMAIL"),
		BootstrapAdminPassword: os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"),

		SeedExampleData: os.Getenv("SEED_EXAMPLE_DATA") == "true",

		SMTPHost: os.Getenv("SMTP_HOST"),
		SMTPPort: getenv("SMTP_PORT", "587"),
		SMTPUser: os.Getenv("SMTP_USER"),
		SMTPPass: os.Getenv("SMTP_PASS"),
		SMTPFrom: getenv("SMTP_FROM", "studio@localhost"),

		LogLevel:  getenv("LOG_LEVEL", "info"),
		LogFormat: getenv("LOG_FORMAT", "text"),
		LogDir:    getenv("LOG_DIR", "./data/log"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
