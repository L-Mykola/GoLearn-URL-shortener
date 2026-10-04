package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/time/rate"

	"golearn/internal/config"
	"golearn/internal/httpapi"
	"golearn/internal/link"
	"golearn/internal/memstore"
	"golearn/internal/sqlitestore"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, closeStore, err := openStore(ctx, cfg)
	if err != nil {
		return err
	}
	defer closeStore()
	logger.Info("storage ready", "type", cfg.Storage, "path", cfg.DBPath)

	var wg sync.WaitGroup
	bgCtx, stopBackground := context.WithCancel(ctx)
	defer func() {
		stopBackground()
		wg.Wait()
	}()

	cleaner := link.NewCleaner(store, cfg.CleanupInterval, logger)
	wg.Go(func() { cleaner.Run(bgCtx) })

	svc := link.NewService(store)
	handler := httpapi.New(svc, logger, httpapi.Options{
		BaseURL:   cfg.BaseURL,
		RateLimit: rate.Limit(cfg.RateLimit),
		RateBurst: cfg.RateBurst,
	})

	srv := &http.Server{
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	logger.Info("server started", "addr", ln.Addr().String(), "base_url", cfg.BaseURL)

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.Serve(ln)
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}

	logger.Info("server stopped")
	return nil
}

func openStore(ctx context.Context, cfg config.Config) (link.Store, func() error, error) {
	switch cfg.Storage {
	case "memory":
		return memstore.New(), func() error { return nil }, nil
	default:
		s, err := sqlitestore.Open(ctx, cfg.DBPath)
		if err != nil {
			return nil, nil, fmt.Errorf("open sqlite: %w", err)
		}
		return s, s.Close, nil
	}
}
