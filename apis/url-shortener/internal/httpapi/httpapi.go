// Package httpapi exposes the URL shortener over HTTP.
//
// Routes:
//
//	POST /api/links              create a short link
//	GET  /api/links/{code}/stats usage counters of a link
//	GET  /{code}                 302 redirect to the original URL
//
// Errors are always returned as JSON objects of the form
//
//	{"error": {"code": "invalid_url", "message": "..."}}
//
// where code is a stable, machine-readable identifier.
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/brunofbenicio/go-lab/apis/url-shortener/internal/base62"
	"github.com/brunofbenicio/go-lab/apis/url-shortener/internal/shortener"
	"github.com/brunofbenicio/go-lab/apis/url-shortener/internal/store"
)

// MaxBodyBytes is the largest accepted request body.
const MaxBodyBytes = 8 << 10

// Service is the behavior the HTTP layer needs from the shortener.
// *shortener.Service satisfies it.
type Service interface {
	Shorten(ctx context.Context, in shortener.ShortenInput) (store.Link, error)
	Resolve(ctx context.Context, code string) (store.Link, error)
	Stats(ctx context.Context, code string) (shortener.Stats, error)
}

// Handler serves the shortener API. Create it with New and mount it with
// Register.
type Handler struct {
	svc     Service
	baseURL string
	logger  *slog.Logger
}

// Option customizes a Handler.
type Option func(*Handler)

// WithBaseURL sets the public origin used to build short URLs, for example
// "https://sho.example.com". When empty, the origin is derived from each
// request's Host header.
func WithBaseURL(baseURL string) Option {
	return func(h *Handler) { h.baseURL = strings.TrimRight(baseURL, "/") }
}

// WithLogger sets the logger used to report unexpected failures. A nil logger
// is ignored. By default nothing is logged.
func WithLogger(l *slog.Logger) Option {
	return func(h *Handler) {
		if l != nil {
			h.logger = l
		}
	}
}

// New returns a Handler backed by svc.
func New(svc Service, opts ...Option) *Handler {
	h := &Handler{svc: svc, logger: slog.New(slog.DiscardHandler)}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// route couples a ServeMux pattern with its handler method. Keeping both in one
// table guarantees that Routes and Register can never disagree.
type route struct {
	pattern string
	handle  func(*Handler, http.ResponseWriter, *http.Request)
}

var routes = []route{
	{"POST /api/links", (*Handler).createLink},
	{"GET /api/links/{code}/stats", (*Handler).linkStats},
	{"GET /{code}", (*Handler).redirect},
}

// Routes lists the ServeMux patterns registered by Register. It is used to
// check that every route is documented in the OpenAPI specification.
func Routes() []string {
	patterns := make([]string, len(routes))
	for i, r := range routes {
		patterns[i] = r.pattern
	}
	return patterns
}

// Register mounts every route of the API on mux.
func (h *Handler) Register(mux *http.ServeMux) {
	for _, r := range routes {
		mux.HandleFunc(r.pattern, func(w http.ResponseWriter, req *http.Request) {
			r.handle(h, w, req)
		})
	}
}

// createLinkRequest is the body of POST /api/links.
type createLinkRequest struct {
	URL        string `json:"url"`
	TTLSeconds *int64 `json:"ttl_seconds"`
}

// linkResponse is the representation of a freshly created link.
type linkResponse struct {
	Code      string     `json:"code"`
	ShortURL  string     `json:"short_url"`
	URL       string     `json:"url"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// statsResponse is the representation of GET /api/links/{code}/stats.
type statsResponse struct {
	Code         string     `json:"code"`
	URL          string     `json:"url"`
	CreatedAt    time.Time  `json:"created_at"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	Hits         int64      `json:"hits"`
	LastAccessAt *time.Time `json:"last_access_at,omitempty"`
	Expired      bool       `json:"expired"`
}

// errorBody is the envelope of every error response.
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (h *Handler) createLink(w http.ResponseWriter, r *http.Request) {
	var req createLinkRequest
	if !h.decodeJSON(w, r, &req) {
		return
	}

	var ttl time.Duration
	if req.TTLSeconds != nil {
		if *req.TTLSeconds < 1 {
			h.writeError(w, http.StatusUnprocessableEntity, "invalid_ttl", "ttl_seconds must be a positive integer")
			return
		}
		ttl = secondsToDuration(*req.TTLSeconds)
	}

	link, err := h.svc.Shorten(r.Context(), shortener.ShortenInput{URL: req.URL, TTL: ttl})
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}

	w.Header().Set("Location", h.origin(r)+"/api/links/"+link.Code+"/stats")
	h.writeJSON(w, http.StatusCreated, linkResponse{
		Code:      link.Code,
		ShortURL:  h.origin(r) + "/" + link.Code,
		URL:       link.URL,
		CreatedAt: link.CreatedAt,
		ExpiresAt: optionalTime(link.ExpiresAt),
	})
}

func (h *Handler) linkStats(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if !validCode(code) {
		h.writeError(w, http.StatusNotFound, "not_found", "link not found")
		return
	}

	stats, err := h.svc.Stats(r.Context(), code)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	h.writeJSON(w, http.StatusOK, statsResponse{
		Code:         stats.Code,
		URL:          stats.URL,
		CreatedAt:    stats.CreatedAt,
		ExpiresAt:    optionalTime(stats.ExpiresAt),
		Hits:         stats.Hits,
		LastAccessAt: optionalTime(stats.LastAccessAt),
		Expired:      stats.Expired,
	})
}

func (h *Handler) redirect(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if !validCode(code) {
		h.writeError(w, http.StatusNotFound, "not_found", "link not found")
		return
	}

	link, err := h.svc.Resolve(r.Context(), code)
	if err != nil {
		h.writeServiceError(w, r, err)
		return
	}
	// A 302 is not cached by default; no-store makes it explicit so every
	// access reaches the server and is counted.
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, link.URL, http.StatusFound)
}

// validCode reports whether code can possibly name a link, so malformed paths
// (such as /favicon.ico) are rejected without touching the store.
func validCode(code string) bool {
	return len(code) >= shortener.MinCodeLength &&
		len(code) <= shortener.MaxCodeLength &&
		base62.IsValid(code)
}

// origin returns the public origin used in absolute URLs: the configured base
// URL or, failing that, the scheme and host of the incoming request.
func (h *Handler) origin(r *http.Request) string {
	if h.baseURL != "" {
		return h.baseURL
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// decodeJSON validates the content type and decodes exactly one JSON object
// from the request body into dst. On failure it writes the error response and
// returns false.
func (h *Handler) decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		h.writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type",
			"Content-Type must be application/json")
		return false
	}

	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		h.writeDecodeError(w, err)
		return false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "request body must contain a single JSON object")
		return false
	}
	return true
}

func (h *Handler) writeDecodeError(w http.ResponseWriter, err error) {
	var (
		syntaxErr *json.SyntaxError
		typeErr   *json.UnmarshalTypeError
		tooBigErr *http.MaxBytesError
	)
	switch {
	case errors.As(err, &tooBigErr):
		h.writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large",
			"request body must not exceed 8 KiB")
	case errors.Is(err, io.EOF):
		h.writeError(w, http.StatusBadRequest, "invalid_json", "request body must not be empty")
	case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
		h.writeError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
	case errors.As(err, &typeErr) && typeErr.Field == "":
		h.writeError(w, http.StatusBadRequest, "invalid_json", "request body must be a JSON object")
	case errors.As(err, &typeErr):
		h.writeError(w, http.StatusBadRequest, "invalid_json",
			"field \""+typeErr.Field+"\" has the wrong type")
	default:
		// Unknown fields and any other decoding problem.
		h.writeError(w, http.StatusBadRequest, "invalid_json", strings.TrimPrefix(err.Error(), "json: "))
	}
}

// writeServiceError translates service errors to HTTP responses. Unexpected
// errors are logged and reported without internal details.
func (h *Handler) writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, shortener.ErrInvalidURL):
		h.writeError(w, http.StatusUnprocessableEntity, "invalid_url", err.Error())
	case errors.Is(err, shortener.ErrInvalidTTL):
		h.writeError(w, http.StatusUnprocessableEntity, "invalid_ttl", err.Error())
	case errors.Is(err, shortener.ErrNotFound):
		h.writeError(w, http.StatusNotFound, "not_found", "link not found")
	case errors.Is(err, shortener.ErrExpired):
		h.writeError(w, http.StatusGone, "link_expired", "this link has expired")
	case errors.Is(err, shortener.ErrCodeGeneration):
		h.logger.Warn("code generation exhausted", "path", r.URL.Path, "error", err)
		h.writeError(w, http.StatusServiceUnavailable, "code_generation_failed",
			"could not generate a unique code, please try again")
	default:
		h.logger.Error("unexpected service error", "method", r.Method, "path", r.URL.Path, "error", err)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	h.writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}

// writeJSON encodes v before touching the response so that an encoding failure
// can still produce a clean 500.
func (h *Handler) writeJSON(w http.ResponseWriter, status int, v any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		h.logger.Error("encode response", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if _, err := w.Write(buf.Bytes()); err != nil {
		h.logger.Debug("write response", "error", err)
	}
}

// optionalTime returns nil for the zero time so that the field is omitted from
// the JSON output.
func optionalTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// secondsToDuration converts seconds to a Duration, saturating instead of
// overflowing; the service then rejects the saturated value as too long.
func secondsToDuration(secs int64) time.Duration {
	if secs > math.MaxInt64/int64(time.Second) {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(secs) * time.Second
}
