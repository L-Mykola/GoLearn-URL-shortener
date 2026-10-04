package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLogRequests(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("hello"))
	})

	rec := httptest.NewRecorder()
	logRequests(log, next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log is not JSON: %v: %s", err, buf.String())
	}
	if entry["status"] != float64(http.StatusTeapot) {
		t.Errorf("status = %v", entry["status"])
	}
	if entry["bytes"] != float64(5) {
		t.Errorf("bytes = %v", entry["bytes"])
	}
	if entry["path"] != "/x" || entry["method"] != "GET" {
		t.Errorf("unexpected entry: %v", entry)
	}
}

func TestLogRequestsImplicitOK(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	logRequests(log, next).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	var entry map[string]any
	json.Unmarshal(buf.Bytes(), &entry)
	if entry["status"] != float64(http.StatusOK) {
		t.Errorf("status = %v, want 200", entry["status"])
	}
}

func TestRecoverPanic(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	rec := httptest.NewRecorder()
	recoverPanic(log, next).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"panic":"boom"`)) {
		t.Errorf("panic not logged: %s", buf.String())
	}
}
