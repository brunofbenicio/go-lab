// Package httpapi exposes the webhook receiver over HTTP.
//
// Route:
//
//	POST /webhooks  receive a signed event
//
// Every delivery must carry three headers:
//
//	X-Webhook-Id         unique ID of the delivery, reused by retries
//	X-Webhook-Timestamp  Unix time of the attempt, in seconds
//	X-Webhook-Signature  "sha256=" + hex HMAC-SHA-256 of "<id>.<timestamp>.<body>"
//
// The body must be a JSON object with a "type" and an optional "data".
//
// A delivery goes through these steps, and the first failure ends the request:
//
//  1. body size limit (413)
//  2. presence of the signature (401), the ID and the timestamp (400)
//  3. signature verification (401)
//  4. timestamp tolerance (400 if unparsable, 401 if too far from now)
//  5. payload validation (400)
//  6. duplicate detection by delivery ID (409)
//  7. processing (202 on success, 500 on failure; the ID is then released so
//     that the sender's retry is processed)
//
// Nothing that happens before step 3 completes can consume a delivery ID, and
// nothing after it is revealed to unauthenticated callers. Errors are JSON
// objects of the form {"error": {"code": "...", "message": "..."}}.
package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/brunofbenicio/go-lab/apis/webhook-receiver/internal/idempotency"
	"github.com/brunofbenicio/go-lab/apis/webhook-receiver/internal/replay"
	"github.com/brunofbenicio/go-lab/apis/webhook-receiver/internal/signature"
)

// Header names of a delivery.
const (
	HeaderID        = "X-Webhook-Id"
	HeaderTimestamp = "X-Webhook-Timestamp"
	HeaderSignature = "X-Webhook-Signature"
)

const (
	// DefaultMaxBodyBytes is the largest accepted body unless overridden.
	DefaultMaxBodyBytes = 64 << 10

	maxIDLength   = 128
	maxTypeLength = 100
)

var (
	idPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]*$`)
	typePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)
)

// Event is a validated, authenticated delivery handed to the Processor.
type Event struct {
	// ID is the delivery ID from the X-Webhook-Id header.
	ID string
	// Type is the event type, such as "order.created".
	Type string
	// Data is the raw JSON of the "data" field, nil when it was absent.
	Data json.RawMessage
	// ReceivedAt is when the delivery was accepted.
	ReceivedAt time.Time
}

// Processor does the actual work for an accepted event. Returning an error
// makes the receiver answer 500 and forget the delivery ID, so the sender's
// retry is processed again.
type Processor interface {
	Process(ctx context.Context, event Event) error
}

// ProcessorFunc adapts a function to the Processor interface.
type ProcessorFunc func(ctx context.Context, event Event) error

// Process calls f.
func (f ProcessorFunc) Process(ctx context.Context, event Event) error { return f(ctx, event) }

// Deduper remembers delivery IDs. *idempotency.Store satisfies it.
type Deduper interface {
	// Claim returns true if the caller is the first to register key.
	Claim(key string) bool
	// Release forgets key.
	Release(key string)
}

// TimestampChecker validates delivery timestamps. *replay.Checker satisfies it.
type TimestampChecker interface {
	Check(timestamp string) error
}

// Config holds the collaborators of the Handler.
type Config struct {
	// Secret is the shared key used to verify signatures. It is required.
	Secret []byte
	// Dedupe detects repeated deliveries. It is required.
	Dedupe Deduper
	// Timestamps rejects stale and future-dated deliveries. It is required.
	Timestamps TimestampChecker
	// Processor handles accepted events. It is required.
	Processor Processor
}

// Handler serves the webhook endpoint. Create it with New and mount it with
// Register.
type Handler struct {
	secret     []byte
	dedupe     Deduper
	timestamps TimestampChecker
	processor  Processor

	maxBody int64
	now     func() time.Time
	logger  *slog.Logger
}

// Option customizes a Handler.
type Option func(*Handler)

// WithMaxBodyBytes sets the largest accepted body. Non-positive values are
// ignored.
func WithMaxBodyBytes(n int64) Option {
	return func(h *Handler) {
		if n > 0 {
			h.maxBody = n
		}
	}
}

// WithClock replaces the time source used to stamp events. A nil function is
// ignored.
func WithClock(now func() time.Time) Option {
	return func(h *Handler) {
		if now != nil {
			h.now = now
		}
	}
}

// WithLogger sets the logger used to report processing failures. A nil logger
// is ignored. By default nothing is logged.
func WithLogger(l *slog.Logger) Option {
	return func(h *Handler) {
		if l != nil {
			h.logger = l
		}
	}
}

// New builds a Handler. It fails if a collaborator or the secret is missing.
func New(cfg Config, opts ...Option) (*Handler, error) {
	if len(cfg.Secret) == 0 {
		return nil, errors.New("httpapi: secret is required")
	}
	if cfg.Dedupe == nil || cfg.Timestamps == nil || cfg.Processor == nil {
		return nil, errors.New("httpapi: dedupe, timestamps and processor are all required")
	}
	h := &Handler{
		secret:     append([]byte(nil), cfg.Secret...),
		dedupe:     cfg.Dedupe,
		timestamps: cfg.Timestamps,
		processor:  cfg.Processor,
		maxBody:    DefaultMaxBodyBytes,
		now:        time.Now,
		logger:     slog.New(slog.DiscardHandler),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h, nil
}

// route couples a ServeMux pattern with its handler method, so Routes and
// Register share a single source of truth.
type route struct {
	pattern string
	handle  func(*Handler, http.ResponseWriter, *http.Request)
}

var routes = []route{
	{"POST /webhooks", (*Handler).receive},
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

// acceptedResponse is the body of a 202.
type acceptedResponse struct {
	Status string `json:"status"`
	ID     string `json:"id"`
}

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (h *Handler) receive(w http.ResponseWriter, r *http.Request) {
	// 1. Size limit. A declared length is checked before reading anything; the
	//    reader below enforces it for chunked bodies too.
	if r.ContentLength > h.maxBody {
		h.tooLarge(w)
		return
	}

	// 2. Required headers.
	sig := r.Header.Get(HeaderSignature)
	if sig == "" {
		h.writeError(w, http.StatusUnauthorized, "missing_signature", HeaderSignature+" header is required")
		return
	}
	id := r.Header.Get(HeaderID)
	timestamp := r.Header.Get(HeaderTimestamp)
	switch {
	case id == "":
		h.writeError(w, http.StatusBadRequest, "missing_header", HeaderID+" header is required")
		return
	case timestamp == "":
		h.writeError(w, http.StatusBadRequest, "missing_header", HeaderTimestamp+" header is required")
		return
	case len(id) > maxIDLength || !idPattern.MatchString(id):
		h.writeError(w, http.StatusBadRequest, "invalid_webhook_id",
			fmt.Sprintf("%s must have 1 to %d characters: letters, digits, '.', '_', ':' or '-'", HeaderID, maxIDLength))
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, h.maxBody))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			h.tooLarge(w)
			return
		}
		h.writeError(w, http.StatusBadRequest, "invalid_payload", "could not read the request body")
		return
	}

	// 3. Authentication. Every failure looks the same to the caller.
	if err := signature.Verify(h.secret, signature.Message(id, timestamp, body), sig); err != nil {
		h.writeError(w, http.StatusUnauthorized, "invalid_signature", "signature verification failed")
		return
	}

	// 4. Replay protection. The timestamp is authentic now, so the sender may
	//    be told precisely what is wrong with it.
	if err := h.timestamps.Check(timestamp); err != nil {
		switch {
		case errors.Is(err, replay.ErrTooOld), errors.Is(err, replay.ErrTooNew):
			h.writeError(w, http.StatusUnauthorized, "timestamp_out_of_tolerance",
				"timestamp is too far from the current time; check the sender's clock or retry with a fresh signature")
		default:
			h.writeError(w, http.StatusBadRequest, "invalid_timestamp",
				HeaderTimestamp+" must be a Unix time in seconds, such as 1740830400")
		}
		return
	}

	// 5. Payload.
	payload, problem := parsePayload(body)
	if problem != "" {
		h.writeError(w, http.StatusBadRequest, "invalid_payload", problem)
		return
	}

	// 6. Duplicates.
	if !h.dedupe.Claim(id) {
		h.writeError(w, http.StatusConflict, "duplicate_delivery", "this delivery has already been received")
		return
	}

	// 7. Processing.
	event := Event{ID: id, Type: payload.Type, Data: payload.Data, ReceivedAt: h.now().UTC()}
	if err := h.processor.Process(r.Context(), event); err != nil {
		h.dedupe.Release(id)
		h.logger.Error("processing webhook failed", "delivery", id, "type", event.Type, "error", err)
		h.writeError(w, http.StatusInternalServerError, "processing_failed",
			"the event could not be processed; retry the delivery")
		return
	}
	h.writeJSON(w, http.StatusAccepted, acceptedResponse{Status: "accepted", ID: id})
}

// payload is the validated body of a delivery.
type payload struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// parsePayload validates the body and returns the event fields. When the body
// is unacceptable it returns a message for the sender instead.
func parsePayload(body []byte) (payload, string) {
	if len(bytes.TrimSpace(body)) == 0 {
		return payload{}, "request body must not be empty"
	}

	var p payload
	if err := json.Unmarshal(body, &p); err != nil {
		var typeErr *json.UnmarshalTypeError
		switch {
		case errors.As(err, &typeErr) && typeErr.Field == "":
			return payload{}, "request body must be a JSON object"
		case errors.As(err, &typeErr):
			return payload{}, fmt.Sprintf("field %q has the wrong type", typeErr.Field)
		default:
			return payload{}, "request body is not valid JSON"
		}
	}

	switch {
	case p.Type == "":
		return payload{}, `field "type" is required`
	case len(p.Type) > maxTypeLength || !typePattern.MatchString(p.Type):
		return payload{}, fmt.Sprintf(`field "type" must have 1 to %d characters: lowercase letters, digits, '.', '_' or '-'`, maxTypeLength)
	}
	if bytes.Equal(bytes.TrimSpace(p.Data), []byte("null")) {
		p.Data = nil
	}
	return p, ""
}

func (h *Handler) tooLarge(w http.ResponseWriter) {
	h.writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large",
		fmt.Sprintf("request body must not exceed %d bytes", h.maxBody))
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

// Compile-time checks that the concrete packages satisfy the interfaces.
var (
	_ Deduper          = (*idempotency.Store)(nil)
	_ TimestampChecker = (*replay.Checker)(nil)
)
