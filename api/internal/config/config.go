package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config holds all configuration values loaded from environment variables.
type Config struct {
	DatabaseURL    string
	AppDatabaseURL string
	JWTSecret      string
	Port           string

	BaseHost string

	SMTPHost string
	SMTPPort string
	SMTPUser string
	SMTPPass string

	R2AccountID string
	R2AccessKey string
	R2SecretKey string
	R2Bucket    string
}

// Load reads a .env file (if present) and populates Config from environment
// variables. Missing .env is not an error — production containers typically
// inject env vars directly.
func Load() (*Config, error) {
	// Ignore error: .env is optional (e.g. in production).
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	cfg := &Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		Port:        port,

		SMTPHost: os.Getenv("SMTP_HOST"),
		SMTPPort: os.Getenv("SMTP_PORT"),
		SMTPUser: os.Getenv("SMTP_USER"),
		SMTPPass: os.Getenv("SMTP_PASS"),

		R2AccountID: os.Getenv("R2_ACCOUNT_ID"),
		R2AccessKey: os.Getenv("R2_ACCESS_KEY"),
		R2SecretKey: os.Getenv("R2_SECRET_KEY"),
		R2Bucket:    os.Getenv("R2_BUCKET"),
	}

	cfg.AppDatabaseURL = os.Getenv("APP_DATABASE_URL")
	if cfg.AppDatabaseURL == "" {
		cfg.AppDatabaseURL = cfg.DatabaseURL
	}
	cfg.BaseHost = os.Getenv("BASE_HOST")
	if cfg.BaseHost == "" {
		cfg.BaseHost = "mutqin.app"
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL environment variable is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET environment variable is required")
	}

	return cfg, nil
}
