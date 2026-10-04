package sqlitestore

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

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

func TestMigratesV1Database(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.db")

	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(migrations[0] + `;
		INSERT INTO links (code, url, created_at, visits) VALUES ('old', 'https://go.dev', '2026-01-01T00:00:00.5Z', 3);
		PRAGMA user_version = 1;`)
	db.Close()
	if err != nil {
		t.Fatal(err)
	}

	s, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	got, err := s.Get(ctx, "old")
	if err != nil {
		t.Fatal(err)
	}
	if got.Visits != 3 || !got.ExpiresAt.IsZero() {
		t.Errorf("unexpected link after migration: %+v", got)
	}

	var version int
	s.db.QueryRow(`PRAGMA user_version`).Scan(&version)
	if version != len(migrations) {
		t.Errorf("user_version = %d, want %d", version, len(migrations))
	}
}

func TestTimeLayoutSortsLexicographically(t *testing.T) {
	a := time.Date(2026, 1, 1, 0, 0, 5, 0, time.UTC)
	b := a.Add(500 * time.Millisecond)

	if !(formatTime(a) < formatTime(b)) {
		t.Errorf("%s should sort before %s", formatTime(a), formatTime(b))
	}
}
