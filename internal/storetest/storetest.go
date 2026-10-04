package storetest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"golearn/internal/link"
)

func Run(t *testing.T, newStore func(t *testing.T) link.Store) {
	t.Run("SaveAndGet", func(t *testing.T) {
		ctx := context.Background()
		s := newStore(t)
		want := link.Link{
			Code:      "abc",
			URL:       "https://go.dev",
			CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 6, time.UTC),
		}

		if err := s.Save(ctx, want); err != nil {
			t.Fatal(err)
		}
		got, err := s.Get(ctx, "abc")
		if err != nil {
			t.Fatal(err)
		}
		if got.Code != want.Code || got.URL != want.URL || got.Visits != 0 || !got.CreatedAt.Equal(want.CreatedAt) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("SaveDuplicate", func(t *testing.T) {
		ctx := context.Background()
		s := newStore(t)
		if err := s.Save(ctx, link.Link{Code: "abc", URL: "https://a.com"}); err != nil {
			t.Fatal(err)
		}

		if err := s.Save(ctx, link.Link{Code: "abc", URL: "https://b.com"}); !errors.Is(err, link.ErrExists) {
			t.Fatalf("want ErrExists, got %v", err)
		}
		got, _ := s.Get(ctx, "abc")
		if got.URL != "https://a.com" {
			t.Errorf("original link overwritten: %q", got.URL)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		ctx := context.Background()
		s := newStore(t)

		if _, err := s.Get(ctx, "nope"); !errors.Is(err, link.ErrNotFound) {
			t.Errorf("Get: want ErrNotFound, got %v", err)
		}
		if err := s.IncrementVisits(ctx, "nope"); !errors.Is(err, link.ErrNotFound) {
			t.Errorf("IncrementVisits: want ErrNotFound, got %v", err)
		}
	})

	t.Run("ConcurrentIncrement", func(t *testing.T) {
		ctx := context.Background()
		s := newStore(t)
		if err := s.Save(ctx, link.Link{Code: "abc", URL: "https://go.dev"}); err != nil {
			t.Fatal(err)
		}

		const n = 200
		errs := make(chan error, n)
		var wg sync.WaitGroup
		for range n {
			wg.Go(func() {
				errs <- s.IncrementVisits(ctx, "abc")
			})
		}
		wg.Wait()
		close(errs)

		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		got, _ := s.Get(ctx, "abc")
		if got.Visits != n {
			t.Errorf("visits = %d, want %d", got.Visits, n)
		}
	})
}
