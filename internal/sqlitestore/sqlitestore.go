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

const timeLayout = "2006-01-02T15:04:05.000000000Z"

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

	if err := migrate(ctx, db); err != nil {
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
		`INSERT INTO links (code, url, created_at, expires_at, visits) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (code) DO NOTHING`,
		l.Code, l.URL, formatTime(l.CreatedAt), nullTime(l.ExpiresAt), l.Visits,
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
		expiresAt sql.NullString
	)

	err := s.db.QueryRowContext(ctx,
		`SELECT code, url, created_at, expires_at, visits FROM links WHERE code = ?`, code,
	).Scan(&l.Code, &l.URL, &createdAt, &expiresAt, &l.Visits)
	if errors.Is(err, sql.ErrNoRows) {
		return link.Link{}, link.ErrNotFound
	}
	if err != nil {
		return link.Link{}, fmt.Errorf("select link: %w", err)
	}

	if l.CreatedAt, err = parseTime(createdAt); err != nil {
		return link.Link{}, fmt.Errorf("parse created_at: %w", err)
	}
	if expiresAt.Valid {
		if l.ExpiresAt, err = parseTime(expiresAt.String); err != nil {
			return link.Link{}, fmt.Errorf("parse expires_at: %w", err)
		}
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

func (s *Store) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM links WHERE expires_at IS NOT NULL AND expires_at <= ?`, formatTime(now),
	)
	if err != nil {
		return 0, fmt.Errorf("delete expired: %w", err)
	}
	return res.RowsAffected()
}

func formatTime(t time.Time) string {
	return t.UTC().Format(timeLayout)
}

func nullTime(t time.Time) sql.NullString {
	if t.IsZero() {
		return sql.NullString{}
	}
	return sql.NullString{String: formatTime(t), Valid: true}
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}
