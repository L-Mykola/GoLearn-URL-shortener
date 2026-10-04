package memstore

import (
	"context"
	"errors"
	"sync"
	"testing"

	"golearn/internal/link"
)

func TestSaveAndGet(t *testing.T) {
	ctx := context.Background()
	s := New()
	want := link.Link{Code: "abc", URL: "https://go.dev"}

	if err := s.Save(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestSaveDuplicate(t *testing.T) {
	ctx := context.Background()
	s := New()
	s.Save(ctx, link.Link{Code: "abc"})

	if err := s.Save(ctx, link.Link{Code: "abc"}); !errors.Is(err, link.ErrExists) {
		t.Errorf("want ErrExists, got %v", err)
	}
}

func TestNotFound(t *testing.T) {
	ctx := context.Background()
	s := New()

	if _, err := s.Get(ctx, "nope"); !errors.Is(err, link.ErrNotFound) {
		t.Errorf("Get: want ErrNotFound, got %v", err)
	}
	if err := s.IncrementVisits(ctx, "nope"); !errors.Is(err, link.ErrNotFound) {
		t.Errorf("IncrementVisits: want ErrNotFound, got %v", err)
	}
}

func TestConcurrentIncrement(t *testing.T) {
	ctx := context.Background()
	s := New()
	s.Save(ctx, link.Link{Code: "abc"})

	const n = 1000
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			s.IncrementVisits(ctx, "abc")
		})
	}
	wg.Wait()

	got, _ := s.Get(ctx, "abc")
	if got.Visits != n {
		t.Errorf("visits = %d, want %d", got.Visits, n)
	}
}
