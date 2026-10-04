package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"golearn/internal/link"
)

const maxBodyBytes = 1 << 20

type LinkService interface {
	Shorten(ctx context.Context, rawURL string) (link.Link, error)
	Resolve(ctx context.Context, code string) (string, error)
	Stats(ctx context.Context, code string) (link.Link, error)
}

type Handler struct {
	svc     LinkService
	baseURL string
	log     *slog.Logger
}

func New(svc LinkService, baseURL string, log *slog.Logger) *Handler {
	return &Handler{svc: svc, baseURL: baseURL, log: log}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("POST /api/links", h.createLink)
	mux.HandleFunc("GET /api/links/{code}", h.getStats)
	mux.HandleFunc("GET /{code}", h.redirect)

	return logRequests(h.log, recoverPanic(h.log, mux))
}

type createLinkRequest struct {
	URL string `json:"url"`
}

type linkResponse struct {
	Code      string    `json:"code"`
	ShortURL  string    `json:"short_url"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
	Visits    *int64    `json:"visits,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte("ok"))
}

func (h *Handler) createLink(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req createLinkRequest
	if err := dec.Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	l, err := h.svc.Shorten(r.Context(), req.URL)
	if err != nil {
		h.handleError(w, err)
		return
	}

	h.writeJSON(w, http.StatusCreated, h.toResponse(l, false))
}

func (h *Handler) getStats(w http.ResponseWriter, r *http.Request) {
	l, err := h.svc.Stats(r.Context(), r.PathValue("code"))
	if err != nil {
		h.handleError(w, err)
		return
	}
	h.writeJSON(w, http.StatusOK, h.toResponse(l, true))
}

func (h *Handler) redirect(w http.ResponseWriter, r *http.Request) {
	target, err := h.svc.Resolve(r.Context(), r.PathValue("code"))
	if err != nil {
		h.handleError(w, err)
		return
	}
	http.Redirect(w, r, target, http.StatusFound)
}

func (h *Handler) toResponse(l link.Link, withVisits bool) linkResponse {
	resp := linkResponse{
		Code:      l.Code,
		ShortURL:  h.baseURL + "/" + l.Code,
		URL:       l.URL,
		CreatedAt: l.CreatedAt,
	}
	if withVisits {
		resp.Visits = &l.Visits
	}
	return resp
}

func (h *Handler) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, link.ErrInvalidURL):
		h.writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, link.ErrNotFound):
		h.writeError(w, http.StatusNotFound, "link not found")
	default:
		h.log.Error("internal error", "err", err)
		h.writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		h.log.Error("write json", "err", err)
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, msg string) {
	h.writeJSON(w, status, errorResponse{Error: msg})
}
