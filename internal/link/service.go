package link

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	codeLength  = 7
	maxAttempts = 5
	MaxTTL      = 365 * 24 * time.Hour
)

var (
	ErrInvalidURL  = errors.New("invalid url")
	ErrInvalidTTL  = errors.New("invalid ttl")
	ErrInvalidCode = errors.New("invalid code")
)

var (
	customCodeRe  = regexp.MustCompile(`^[A-Za-z0-9_-]{3,32}$`)
	reservedCodes = map[string]struct{}{
		"api":     {},
		"healthz": {},
	}
)

type ShortenInput struct {
	URL  string
	Code string
	TTL  time.Duration
}

type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

func (s *Service) Shorten(ctx context.Context, in ShortenInput) (Link, error) {
	if err := validateURL(in.URL); err != nil {
		return Link{}, err
	}
	if in.TTL < 0 || in.TTL > MaxTTL {
		return Link{}, fmt.Errorf("%w: must be between 0 and %s", ErrInvalidTTL, MaxTTL)
	}

	if in.Code != "" {
		if err := validateCode(in.Code); err != nil {
			return Link{}, err
		}
	}

	now := s.now().UTC()
	l := Link{
		URL:       in.URL,
		CreatedAt: now,
	}
	if in.TTL > 0 {
		l.ExpiresAt = now.Add(in.TTL)
	}

	if in.Code != "" {
		l.Code = in.Code
		if err := s.store.Save(ctx, l); err != nil {
			if errors.Is(err, ErrExists) {
				return Link{}, ErrExists
			}
			return Link{}, fmt.Errorf("save link: %w", err)
		}
		return l, nil
	}

	for range maxAttempts {
		l.Code = generateCode(codeLength)
		if isReserved(l.Code) {
			continue
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
	l, err := s.get(ctx, code)
	if err != nil {
		return "", err
	}
	if err := s.store.IncrementVisits(ctx, code); err != nil {
		return "", fmt.Errorf("increment visits: %w", err)
	}
	return l.URL, nil
}

func (s *Service) Stats(ctx context.Context, code string) (Link, error) {
	return s.get(ctx, code)
}

func (s *Service) get(ctx context.Context, code string) (Link, error) {
	l, err := s.store.Get(ctx, code)
	if err != nil {
		return Link{}, err
	}
	if l.Expired(s.now()) {
		return Link{}, ErrNotFound
	}
	return l, nil
}

func validateCode(code string) error {
	if !customCodeRe.MatchString(code) {
		return fmt.Errorf("%w: must be 3-32 characters of A-Z, a-z, 0-9, '-' or '_'", ErrInvalidCode)
	}
	if isReserved(code) {
		return fmt.Errorf("%w: %q is reserved", ErrInvalidCode, code)
	}
	return nil
}

func isReserved(code string) bool {
	_, ok := reservedCodes[strings.ToLower(code)]
	return ok
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
