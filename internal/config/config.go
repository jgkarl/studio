// Package config loads runtime configuration from environment variables. Kept as a single flat
// struct read once at startup — no config library, no reflection-based env binding.
package config

import "os"

type Config struct {
	Port            string
	DBPath          string
	AuthSecret      string
	AppURL          string
	AllowDevLogin   bool
	MediaStorageDir string

	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string
	SMTPFrom string
}

func Load() *Config {
	return &Config{
		Port:            getenv("PORT", "3000"),
		DBPath:          getenv("DB_PATH", "./data/studio.db"),
		AuthSecret:      getenv("AUTH_SECRET", "dev-only-insecure-secret-change-me"),
		AppURL:          os.Getenv("APP_URL"),
		AllowDevLogin:   os.Getenv("ALLOW_DEV_LOGIN") == "true",
		MediaStorageDir: getenv("MEDIA_STORAGE_DIR", "./data/media-storage"),
		SMTPHost:        os.Getenv("SMTP_HOST"),
		SMTPPort:        getenv("SMTP_PORT", "587"),
		SMTPUser:        os.Getenv("SMTP_USER"),
		SMTPPass:        os.Getenv("SMTP_PASS"),
		SMTPFrom:        getenv("SMTP_FROM", "studio@localhost"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
