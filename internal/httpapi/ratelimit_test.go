package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"golang.org/x/time/rate"

	"golearn/internal/link"
)

func TestRateLimitCreateLink(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		svc := &fakeService{
			shortenFn: func(_ context.Context, in link.ShortenInput) (link.Link, error) {
				return link.Link{Code: "abc", URL: in.URL}, nil
			},
			resolveFn: func(_ context.Context, code string) (string, error) {
				return "https://go.dev", nil
			},
		}
		routes := New(svc, slog.New(slog.DiscardHandler), Options{
			BaseURL:   baseURL,
			RateLimit: rate.Every(2 * time.Second),
			RateBurst: 2,
		}).Routes()

		post := func(ip string) *httptest.ResponseRecorder {
			req := httptest.NewRequest(http.MethodPost, "/api/links", strings.NewReader(`{"url":"https://go.dev"}`))
			req.RemoteAddr = ip + ":12345"
			rec := httptest.NewRecorder()
			routes.ServeHTTP(rec, req)
			return rec
		}

		for i := range 2 {
			if rec := post("10.0.0.1"); rec.Code != http.StatusCreated {
				t.Fatalf("request %d: status = %d, want 201", i+1, rec.Code)
			}
		}

		rec := post("10.0.0.1")
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("status = %d, want 429", rec.Code)
		}
		if ra := rec.Header().Get("Retry-After"); ra != "2" {
			t.Errorf("Retry-After = %q, want 2", ra)
		}

		if rec := post("10.0.0.2"); rec.Code != http.StatusCreated {
			t.Errorf("other ip: status = %d, want 201", rec.Code)
		}

		redirect := httptest.NewRequest(http.MethodGet, "/abc", nil)
		redirect.RemoteAddr = "10.0.0.1:12345"
		rrec := httptest.NewRecorder()
		routes.ServeHTTP(rrec, redirect)
		if rrec.Code != http.StatusFound {
			t.Errorf("redirect must not be rate limited, status = %d", rrec.Code)
		}

		time.Sleep(2 * time.Second)
		if rec := post("10.0.0.1"); rec.Code != http.StatusCreated {
			t.Errorf("after refill: status = %d, want 201", rec.Code)
		}
	})
}

func TestLimiterRejectedRequestDoesNotConsumeToken(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := newIPLimiter(rate.Every(time.Second), 1)

		if ok, _ := l.allow("ip"); !ok {
			t.Fatal("first request must pass")
		}
		for range 10 {
			if ok, _ := l.allow("ip"); ok {
				t.Fatal("request must be limited")
			}
		}

		time.Sleep(time.Second)
		if ok, _ := l.allow("ip"); !ok {
			t.Error("rejected requests must not push the next token further away")
		}
	})
}

func TestLimiterSweepsIdleVisitors(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := newIPLimiter(rate.Every(time.Second), 1)
		l.allow("old")

		time.Sleep(visitorIdleTTL + sweepInterval)
		l.allow("new")

		if _, ok := l.visitors["old"]; ok {
			t.Error("idle visitor was not swept")
		}
		if _, ok := l.visitors["new"]; !ok {
			t.Error("active visitor is missing")
		}
	})
}

func TestClientIP(t *testing.T) {
	tests := map[string]string{
		"192.168.1.10:5555": "192.168.1.10",
		"[::1]:8080":        "::1",
		"no-port":           "no-port",
	}
	for addr, want := range tests {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = addr
		if got := clientIP(r); got != want {
			t.Errorf("clientIP(%q) = %q, want %q", addr, got, want)
		}
	}
}
