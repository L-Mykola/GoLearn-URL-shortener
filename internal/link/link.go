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
	Visits    int64
}

type Store interface {
	Save(ctx context.Context, l Link) error
	Get(ctx context.Context, code string) (Link, error)
	IncrementVisits(ctx context.Context, code string) error
}
