package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"
)

type Config struct {
	Addr            string
	BaseURL         string
	LogLevel        slog.Level
	ShutdownTimeout time.Duration
	Storage         string
	DBPath          string
	CleanupInterval time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Addr:            getEnv("ADDR", ":8090"),
		BaseURL:         strings.TrimRight(getEnv("BASE_URL", "http://localhost:8090"), "/"),
		Storage:         getEnv("STORAGE", "sqlite"),
		DBPath:          getEnv("DB_PATH", "shortener.db"),
		ShutdownTimeout: 10 * time.Second,
		CleanupInterval: time.Minute,
	}

	switch cfg.Storage {
	case "sqlite", "memory":
	default:
		return Config{}, fmt.Errorf("STORAGE: unknown value %q, want sqlite or memory", cfg.Storage)
	}

	if err := cfg.LogLevel.UnmarshalText([]byte(getEnv("LOG_LEVEL", "info"))); err != nil {
		return Config{}, fmt.Errorf("LOG_LEVEL: %w", err)
	}

	if err := durationEnv("SHUTDOWN_TIMEOUT", &cfg.ShutdownTimeout); err != nil {
		return Config{}, err
	}
	if err := durationEnv("CLEANUP_INTERVAL", &cfg.CleanupInterval); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func durationEnv(key string, dst *time.Duration) error {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	if d <= 0 {
		return fmt.Errorf("%s: must be positive, got %s", key, d)
	}
	*dst = d
	return nil
}
