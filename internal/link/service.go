package link

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"
)

const (
	codeLength  = 7
	maxAttempts = 5
)

var ErrInvalidURL = errors.New("invalid url")

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

func (s *Service) Shorten(ctx context.Context, rawURL string) (Link, error) {
	if err := validateURL(rawURL); err != nil {
		return Link{}, err
	}

	for range maxAttempts {
		l := Link{
			Code:      generateCode(codeLength),
			URL:       rawURL,
			CreatedAt: s.now().UTC(),
		}

		err := s.store.Save(ctx, l)
		if err == nil {
			return l, nil
		}
		if !errors.Is(err, ErrExists) {
			return Link{}, fmt.Errorf("save link: %w", err)
		}
	}
	return Link{}, fmt.Errorf("generate unique code after %d attempts", maxAttempts)
}

func (s *Service) Resolve(ctx context.Context, code string) (string, error) {
	l, err := s.store.Get(ctx, code)
	if err != nil {
		return "", err
	}
	if err := s.store.IncrementVisits(ctx, code); err != nil {
		return "", fmt.Errorf("increment visits: %w", err)
	}
	return l.URL, nil
}

func (s *Service) Stats(ctx context.Context, code string) (Link, error) {
	return s.store.Get(ctx, code)
}

func validateURL(raw string) error {
	u, err := url.ParseRequestURI(raw)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%w: scheme must be http or https", ErrInvalidURL)
	}
	if u.Host == "" {
		return fmt.Errorf("%w: missing host", ErrInvalidURL)
	}
	return nil
}
