package link

import (
	"context"
	"log/slog"
	"testing"
	"testing/synctest"
	"time"
)

func TestCleanerPurgesOnEveryTick(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := newFakeStore()
		store.links["old"] = Link{Code: "old", ExpiresAt: time.Now().Add(30 * time.Second)}
		store.links["keep"] = Link{Code: "keep"}

		ctx, cancel := context.WithCancel(t.Context())
		c := NewCleaner(store, time.Minute, slog.New(slog.DiscardHandler))

		done := make(chan struct{})
		go func() {
			c.Run(ctx)
			close(done)
		}()

		time.Sleep(3*time.Minute + time.Second)
		synctest.Wait()

		if store.deleteCalls != 3 {
			t.Errorf("delete calls = %d, want 3", store.deleteCalls)
		}
		if _, ok := store.links["old"]; ok {
			t.Error("expired link was not deleted")
		}
		if _, ok := store.links["keep"]; !ok {
			t.Error("link without ttl was deleted")
		}

		cancel()
		<-done
	})
}
