package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr           string
	DataFile           string
	AdminAPIBaseURL    string
	AuditRetentionDays int
	RequestTimeout     time.Duration
	ShutdownTimeout    time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:           getEnv("HTTP_ADDR", ":8080"),
		DataFile:           getEnv("DATA_FILE", "data/app_state.json"),
		AdminAPIBaseURL:    os.Getenv("ADMIN_API_BASE_URL"),
		AuditRetentionDays: getEnvInt("AUDIT_RETENTION_DAYS", 365),
		RequestTimeout:     getEnvDuration("REQUEST_TIMEOUT", 3*time.Second),
		ShutdownTimeout:    getEnvDuration("SHUTDOWN_TIMEOUT", 10*time.Second),
	}
	if cfg.AuditRetentionDays <= 0 {
		return Config{}, fmt.Errorf("AUDIT_RETENTION_DAYS must be positive")
	}
	if cfg.RequestTimeout <= 0 {
		return Config{}, fmt.Errorf("REQUEST_TIMEOUT must be positive")
	}
	if cfg.ShutdownTimeout <= 0 {
		return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT must be positive")
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
