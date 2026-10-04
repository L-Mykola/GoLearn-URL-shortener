package link

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound = errors.New("link not found")
	ErrExists   = errors.New("link already exists")
)

type Link struct {
	Code      string
	URL       string
	CreatedAt time.Time
	ExpiresAt time.Time
	Visits    int64
}

func (l Link) Expired(now time.Time) bool {
	return !l.ExpiresAt.IsZero() && !now.Before(l.ExpiresAt)
}

type Store interface {
	Save(ctx context.Context, l Link) error
	Get(ctx context.Context, code string) (Link, error)
	IncrementVisits(ctx context.Context, code string) error
	DeleteExpired(ctx context.Context, now time.Time) (int64, error)
}
