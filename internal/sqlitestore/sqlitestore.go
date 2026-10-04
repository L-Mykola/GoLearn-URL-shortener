package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"golearn/internal/link"
)

var _ link.Store = (*Store)(nil)

const schema = `
CREATE TABLE IF NOT EXISTS links (
	code       TEXT PRIMARY KEY,
	url        TEXT    NOT NULL,
	created_at TEXT    NOT NULL,
	visits     INTEGER NOT NULL DEFAULT 0
);`

type Store struct {
	db *sql.DB
}

func Open(ctx context.Context, path string) (*Store, error) {
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}

	if _, err := db.ExecContext(ctx, schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Save(ctx context.Context, l link.Link) error {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO links (code, url, created_at, visits) VALUES (?, ?, ?, ?)
		 ON CONFLICT (code) DO NOTHING`,
		l.Code, l.URL, l.CreatedAt.UTC().Format(time.RFC3339Nano), l.Visits,
	)
	if err != nil {
		return fmt.Errorf("insert link: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return link.ErrExists
	}
	return nil
}

func (s *Store) Get(ctx context.Context, code string) (link.Link, error) {
	var (
		l         link.Link
		createdAt string
	)

	err := s.db.QueryRowContext(ctx,
		`SELECT code, url, created_at, visits FROM links WHERE code = ?`, code,
	).Scan(&l.Code, &l.URL, &createdAt, &l.Visits)
	if errors.Is(err, sql.ErrNoRows) {
		return link.Link{}, link.ErrNotFound
	}
	if err != nil {
		return link.Link{}, fmt.Errorf("select link: %w", err)
	}

	l.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return link.Link{}, fmt.Errorf("parse created_at %q: %w", createdAt, err)
	}
	return l, nil
}

func (s *Store) IncrementVisits(ctx context.Context, code string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE links SET visits = visits + 1 WHERE code = ?`, code,
	)
	if err != nil {
		return fmt.Errorf("update visits: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return link.ErrNotFound
	}
	return nil
}
