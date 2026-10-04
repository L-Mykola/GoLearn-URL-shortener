package sqlitestore

import (
	"context"
	"path/filepath"
	"testing"

	"golearn/internal/link"
	"golearn/internal/storetest"
)

func TestStore(t *testing.T) {
	storetest.Run(t, func(t *testing.T) link.Store {
		return openTemp(t)
	})
}

func TestPersistsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save(ctx, link.Link{Code: "abc", URL: "https://go.dev"}); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s, err = Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if _, err := s.Get(ctx, "abc"); err != nil {
		t.Errorf("link lost after reopen: %v", err)
	}
}

func openTemp(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
