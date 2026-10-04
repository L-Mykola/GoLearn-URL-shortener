package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golearn/internal/link"
)

type fakeService struct {
	shortenFn func(ctx context.Context, rawURL string) (link.Link, error)
	resolveFn func(ctx context.Context, code string) (string, error)
	statsFn   func(ctx context.Context, code string) (link.Link, error)
}

func (f *fakeService) Shorten(ctx context.Context, rawURL string) (link.Link, error) {
	return f.shortenFn(ctx, rawURL)
}

func (f *fakeService) Resolve(ctx context.Context, code string) (string, error) {
	return f.resolveFn(ctx, code)
}

func (f *fakeService) Stats(ctx context.Context, code string) (link.Link, error) {
	return f.statsFn(ctx, code)
}

const baseURL = "http://short.test"

func serve(svc LinkService, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	New(svc, baseURL).Routes().ServeHTTP(rec, req)
	return rec
}

func TestCreateLink(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		body       string
		shortenErr error
		wantStatus int
	}{
		{"ok", `{"url":"https://go.dev"}`, nil, http.StatusCreated},
		{"invalid url", `{"url":"ftp://x"}`, link.ErrInvalidURL, http.StatusBadRequest},
		{"broken json", `{"url":`, nil, http.StatusBadRequest},
		{"unknown field", `{"link":"https://go.dev"}`, nil, http.StatusBadRequest},
		{"internal error", `{"url":"https://go.dev"}`, errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{
				shortenFn: func(_ context.Context, rawURL string) (link.Link, error) {
					if tt.shortenErr != nil {
						return link.Link{}, tt.shortenErr
					}
					return link.Link{Code: "abc", URL: rawURL, CreatedAt: created}, nil
				},
			}

			req := httptest.NewRequest(http.MethodPost, "/api/links", strings.NewReader(tt.body))
			rec := serve(svc, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body: %s", rec.Code, tt.wantStatus, rec.Body)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("content-type = %q", ct)
			}
		})
	}
}

func TestCreateLinkResponseBody(t *testing.T) {
	svc := &fakeService{
		shortenFn: func(_ context.Context, rawURL string) (link.Link, error) {
			return link.Link{Code: "abc", URL: rawURL}, nil
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/links", strings.NewReader(`{"url":"https://go.dev"}`))
	rec := serve(svc, req)

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["short_url"] != baseURL+"/abc" {
		t.Errorf("short_url = %v", got["short_url"])
	}
	if _, ok := got["visits"]; ok {
		t.Error("visits must be omitted on create")
	}
}

func TestRedirect(t *testing.T) {
	svc := &fakeService{
		resolveFn: func(_ context.Context, code string) (string, error) {
			if code == "abc" {
				return "https://go.dev", nil
			}
			return "", link.ErrNotFound
		},
	}

	rec := serve(svc, httptest.NewRequest(http.MethodGet, "/abc", nil))
	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusFound)
	}
	if loc := rec.Header().Get("Location"); loc != "https://go.dev" {
		t.Errorf("location = %q", loc)
	}

	rec = serve(svc, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestGetStats(t *testing.T) {
	svc := &fakeService{
		statsFn: func(_ context.Context, code string) (link.Link, error) {
			return link.Link{Code: code, URL: "https://go.dev", Visits: 0}, nil
		},
	}

	rec := serve(svc, httptest.NewRequest(http.MethodGet, "/api/links/abc", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	var got linkResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Visits == nil || *got.Visits != 0 {
		t.Errorf("visits = %v, want 0", got.Visits)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	rec := serve(&fakeService{}, httptest.NewRequest(http.MethodDelete, "/api/links", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
