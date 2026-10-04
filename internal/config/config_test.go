package config

import (
	"log/slog"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	want := Config{
		Addr:            ":8090",
		BaseURL:         "http://localhost:8090",
		LogLevel:        slog.LevelInfo,
		ShutdownTimeout: 10 * time.Second,
		Storage:         "sqlite",
		DBPath:          "shortener.db",
		CleanupInterval: time.Minute,
	}
	if cfg != want {
		t.Errorf("got %+v, want %+v", cfg, want)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("ADDR", ":9000")
	t.Setenv("BASE_URL", "https://sho.rt/")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("SHUTDOWN_TIMEOUT", "3s")
	t.Setenv("STORAGE", "memory")
	t.Setenv("DB_PATH", "/tmp/x.db")
	t.Setenv("CLEANUP_INTERVAL", "30s")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	want := Config{
		Addr:            ":9000",
		BaseURL:         "https://sho.rt",
		LogLevel:        slog.LevelDebug,
		ShutdownTimeout: 3 * time.Second,
		Storage:         "memory",
		DBPath:          "/tmp/x.db",
		CleanupInterval: 30 * time.Second,
	}
	if cfg != want {
		t.Errorf("got %+v, want %+v", cfg, want)
	}
}

func TestLoadInvalid(t *testing.T) {
	tests := []struct {
		key, value string
	}{
		{"LOG_LEVEL", "loud"},
		{"SHUTDOWN_TIMEOUT", "ten"},
		{"STORAGE", "redis"},
		{"CLEANUP_INTERVAL", "0s"},
		{"CLEANUP_INTERVAL", "-1m"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			t.Setenv(tt.key, tt.value)
			if _, err := Load(); err == nil {
				t.Errorf("want error for %s=%q", tt.key, tt.value)
			}
		})
	}
}
