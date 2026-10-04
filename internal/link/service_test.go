package link

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeStore struct {
	links        map[string]Link
	saveErrs     []error
	saveCalls    int
	incrementErr error
	deleteCalls  int
}

func newFakeStore() *fakeStore {
	return &fakeStore{links: make(map[string]Link)}
}

func (f *fakeStore) Save(_ context.Context, l Link) error {
	f.saveCalls++
	if len(f.saveErrs) > 0 {
		err := f.saveErrs[0]
		f.saveErrs = f.saveErrs[1:]
		return err
	}
	f.links[l.Code] = l
	return nil
}

func (f *fakeStore) Get(_ context.Context, code string) (Link, error) {
	l, ok := f.links[code]
	if !ok {
		return Link{}, ErrNotFound
	}
	return l, nil
}

func (f *fakeStore) IncrementVisits(_ context.Context, code string) error {
	if f.incrementErr != nil {
		return f.incrementErr
	}
	l := f.links[code]
	l.Visits++
	f.links[code] = l
	return nil
}

func (f *fakeStore) DeleteExpired(_ context.Context, now time.Time) (int64, error) {
	f.deleteCalls++
	var n int64
	for code, l := range f.links {
		if l.Expired(now) {
			delete(f.links, code)
			n++
		}
	}
	return n, nil
}

func shorten(svc *Service, url string) (Link, error) {
	return svc.Shorten(context.Background(), ShortenInput{URL: url})
}

func TestShortenValidation(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"https", "https://go.dev/doc?x=1", false},
		{"http with port", "http://localhost:8080/path", false},
		{"empty", "", true},
		{"no scheme", "go.dev", true},
		{"ftp scheme", "ftp://example.com", true},
		{"no host", "http://", true},
		{"javascript", "javascript:alert(1)", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(newFakeStore())
			_, err := shorten(svc, tt.url)

			if tt.wantErr && !errors.Is(err, ErrInvalidURL) {
				t.Fatalf("want ErrInvalidURL, got %v", err)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestShortenSetsFields(t *testing.T) {
	fixed := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	store := newFakeStore()
	svc := NewService(store)
	svc.now = func() time.Time { return fixed }

	l, err := shorten(svc, "https://go.dev")
	if err != nil {
		t.Fatal(err)
	}

	if len(l.Code) != codeLength {
		t.Errorf("code length = %d, want %d", len(l.Code), codeLength)
	}
	if l.URL != "https://go.dev" {
		t.Errorf("url = %q", l.URL)
	}
	if !l.CreatedAt.Equal(fixed) {
		t.Errorf("created_at = %v, want %v", l.CreatedAt, fixed)
	}
	if _, ok := store.links[l.Code]; !ok {
		t.Error("link was not saved")
	}
}

func TestShortenRetriesOnCollision(t *testing.T) {
	store := newFakeStore()
	store.saveErrs = []error{ErrExists, ErrExists}
	svc := NewService(store)

	if _, err := shorten(svc, "https://go.dev"); err != nil {
		t.Fatal(err)
	}
	if store.saveCalls != 3 {
		t.Errorf("save calls = %d, want 3", store.saveCalls)
	}
}

func TestShortenGivesUpAfterMaxAttempts(t *testing.T) {
	store := newFakeStore()
	for range maxAttempts {
		store.saveErrs = append(store.saveErrs, ErrExists)
	}
	svc := NewService(store)

	if _, err := shorten(svc, "https://go.dev"); err == nil {
		t.Fatal("want error, got nil")
	}
	if store.saveCalls != maxAttempts {
		t.Errorf("save calls = %d, want %d", store.saveCalls, maxAttempts)
	}
}

func TestShortenWrapsStoreError(t *testing.T) {
	dbErr := errors.New("db is down")
	store := newFakeStore()
	store.saveErrs = []error{dbErr}
	svc := NewService(store)

	_, err := shorten(svc, "https://go.dev")
	if !errors.Is(err, dbErr) {
		t.Fatalf("want wrapped dbErr, got %v", err)
	}
	if store.saveCalls != 1 {
		t.Errorf("save calls = %d, want 1", store.saveCalls)
	}
}

func TestResolve(t *testing.T) {
	store := newFakeStore()
	store.links["abc"] = Link{Code: "abc", URL: "https://go.dev"}
	svc := NewService(store)

	got, err := svc.Resolve(context.Background(), "abc")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://go.dev" {
		t.Errorf("url = %q", got)
	}
	if v := store.links["abc"].Visits; v != 1 {
		t.Errorf("visits = %d, want 1", v)
	}

	if _, err := svc.Resolve(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestShortenTTL(t *testing.T) {
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		ttl         time.Duration
		wantErr     bool
		wantExpires time.Time
	}{
		{"no ttl", 0, false, time.Time{}},
		{"one hour", time.Hour, false, fixed.Add(time.Hour)},
		{"max", MaxTTL, false, fixed.Add(MaxTTL)},
		{"negative", -time.Second, true, time.Time{}},
		{"too long", MaxTTL + time.Second, true, time.Time{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(newFakeStore())
			svc.now = func() time.Time { return fixed }

			l, err := svc.Shorten(context.Background(), ShortenInput{URL: "https://go.dev", TTL: tt.ttl})
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidTTL) {
					t.Fatalf("want ErrInvalidTTL, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !l.ExpiresAt.Equal(tt.wantExpires) {
				t.Errorf("expires_at = %v, want %v", l.ExpiresAt, tt.wantExpires)
			}
		})
	}
}

func TestExpiredLinkIsNotFound(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store := newFakeStore()
	store.links["old"] = Link{Code: "old", URL: "https://go.dev", ExpiresAt: now}
	svc := NewService(store)
	svc.now = func() time.Time { return now }

	if _, err := svc.Resolve(context.Background(), "old"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Resolve: want ErrNotFound, got %v", err)
	}
	if _, err := svc.Stats(context.Background(), "old"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Stats: want ErrNotFound, got %v", err)
	}
	if v := store.links["old"].Visits; v != 0 {
		t.Errorf("visits = %d, expired link must not be counted", v)
	}
}

func TestLinkExpired(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{"never", time.Time{}, false},
		{"future", now.Add(time.Second), false},
		{"exactly now", now, true},
		{"past", now.Add(-time.Second), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (Link{ExpiresAt: tt.expiresAt}).Expired(now); got != tt.want {
				t.Errorf("Expired = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestShortenCustomCode(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		wantErr error
	}{
		{"simple", "my-link", nil},
		{"underscore and digits", "promo_2026", nil},
		{"min length", "abc", nil},
		{"max length", strings.Repeat("a", 32), nil},
		{"too short", "ab", ErrInvalidCode},
		{"too long", strings.Repeat("a", 33), ErrInvalidCode},
		{"space", "my link", ErrInvalidCode},
		{"slash", "a/b", ErrInvalidCode},
		{"cyrillic", "привіт", ErrInvalidCode},
		{"reserved", "healthz", ErrInvalidCode},
		{"reserved any case", "API", ErrInvalidCode},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeStore()
			svc := NewService(store)

			l, err := svc.Shorten(context.Background(), ShortenInput{URL: "https://go.dev", Code: tt.code})
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("want %v, got %v", tt.wantErr, err)
				}
				if store.saveCalls != 0 {
					t.Errorf("store must not be called for invalid code")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if l.Code != tt.code {
				t.Errorf("code = %q, want %q", l.Code, tt.code)
			}
		})
	}
}

func TestShortenCustomCodeTaken(t *testing.T) {
	store := newFakeStore()
	store.saveErrs = []error{ErrExists}
	svc := NewService(store)

	_, err := svc.Shorten(context.Background(), ShortenInput{URL: "https://go.dev", Code: "taken"})
	if !errors.Is(err, ErrExists) {
		t.Fatalf("want ErrExists, got %v", err)
	}
	if store.saveCalls != 1 {
		t.Errorf("save calls = %d, custom code must not be retried", store.saveCalls)
	}
}
