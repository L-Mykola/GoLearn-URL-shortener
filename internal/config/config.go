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
}

func Load() (Config, error) {
	cfg := Config{
		Addr:            getEnv("ADDR", ":8090"),
		BaseURL:         strings.TrimRight(getEnv("BASE_URL", "http://localhost:8090"), "/"),
		ShutdownTimeout: 10 * time.Second,
		Storage:         getEnv("STORAGE", "sqlite"),
		DBPath:          getEnv("DB_PATH", "shortener.db"),
	}

	switch cfg.Storage {
	case "sqlite", "memory":
	default:
		return Config{}, fmt.Errorf("STORAGE: unknown value %q, want sqlite or memory", cfg.Storage)
	}

	if err := cfg.LogLevel.UnmarshalText([]byte(getEnv("LOG_LEVEL", "info"))); err != nil {
		return Config{}, fmt.Errorf("LOG_LEVEL: %w", err)
	}

	if v, ok := os.LookupEnv("SHUTDOWN_TIMEOUT"); ok {
		d, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT: %w", err)
		}
		cfg.ShutdownTimeout = d
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
