package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
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
	RateLimit       float64
	RateBurst       int
}

func Load() (Config, error) {
	cfg := Config{
		Addr:            getEnv("ADDR", ":8080"),
		BaseURL:         strings.TrimRight(getEnv("BASE_URL", "http://localhost:8080"), "/"),
		Storage:         getEnv("STORAGE", "sqlite"),
		DBPath:          getEnv("DB_PATH", "shortener.db"),
		ShutdownTimeout: 10 * time.Second,
		CleanupInterval: time.Minute,
		RateLimit:       1,
		RateBurst:       5,
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

	if v := os.Getenv("RATE_LIMIT_RPS"); v != "" {
		f, err := strconv.ParseFloat(v, 64)
		if err != nil || f < 0 {
			return Config{}, fmt.Errorf("RATE_LIMIT_RPS: want non-negative number, got %q", v)
		}
		cfg.RateLimit = f
	}
	if v := os.Getenv("RATE_LIMIT_BURST"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return Config{}, fmt.Errorf("RATE_LIMIT_BURST: want positive integer, got %q", v)
		}
		cfg.RateBurst = n
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
