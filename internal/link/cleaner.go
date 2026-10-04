package link

import (
	"context"
	"log/slog"
	"time"
)

type Cleaner struct {
	store    Store
	interval time.Duration
	log      *slog.Logger
	now      func() time.Time
}

func NewCleaner(store Store, interval time.Duration, log *slog.Logger) *Cleaner {
	return &Cleaner{store: store, interval: interval, log: log, now: time.Now}
}

func (c *Cleaner) Run(ctx context.Context) {
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	c.log.Info("cleaner started", "interval", c.interval.String())
	for {
		select {
		case <-ctx.Done():
			c.log.Info("cleaner stopped")
			return
		case <-ticker.C:
			c.purge(ctx)
		}
	}
}

func (c *Cleaner) purge(ctx context.Context) {
	n, err := c.store.DeleteExpired(ctx, c.now().UTC())
	if err != nil {
		if ctx.Err() == nil {
			c.log.Error("delete expired links", "err", err)
		}
		return
	}
	if n > 0 {
		c.log.Info("expired links deleted", "count", n)
	}
}
