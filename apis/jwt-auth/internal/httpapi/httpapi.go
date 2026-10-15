// Package httpapi exposes the authentication API over HTTP.
//
// Routes:
//
//	POST /auth/register  create an account
//	POST /auth/login     exchange e-mail and password for a bearer token
//	GET  /me             the authenticated user (requires a bearer token)
//
// Errors are always JSON objects of the form
//
//	{"error": {"code": "email_taken", "message": "..."}}
//
// where code is a stable, machine-readable identifier.
package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/brunofbenicio/go-lab/apis/jwt-auth/internal/jwt"
	"github.com/brunofbenicio/go-lab/apis/jwt-auth/internal/middleware"
	"github.com/brunofbenicio/go-lab/apis/jwt-auth/internal/password"
	"github.com/brunofbenicio/go-lab/apis/jwt-auth/internal/users"
)

// Limits and defaults.
const (
	// MaxBodyBytes is the largest accepted request body.
	MaxBodyBytes = 4 << 10
	// DefaultTokenTTL is the lifetime of access tokens unless overridden.
	DefaultTokenTTL = 15 * time.Minute

	minPasswordChars = 8
	maxPasswordChars = 128
	maxEmailLength   = 254
	maxNameChars     = 100

	// timingPassword is hashed once at startup. Logins for unknown e-mails are
	// verified against it so they cost as much as logins for known ones.
	timingPassword = "password used only to even out login timing"
)

// Tokens issues and verifies access tokens. *jwt.HS256 satisfies it.
type Tokens interface {
	Sign(claims jwt.Claims) (string, error)
	Verify(token string) (jwt.Claims, error)
}

// PasswordHasher hashes and checks passwords. *password.Hasher satisfies it.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(password, encoded string) (bool, error)
}

var (
	_ Tokens         = (*jwt.HS256)(nil)
	_ PasswordHasher = (*password.Hasher)(nil)
)

// Dependencies are the collaborators of the API.
type Dependencies struct {
	Users  users.Store
	Hasher PasswordHasher
	Tokens Tokens
}

// Handler serves the authentication API. Create it with New and mount it with
// Register.
type Handler struct {
	users  users.Store
	hasher PasswordHasher
	tokens Tokens

	now      func() time.Time
	newID    func() (string, error)
	tokenTTL time.Duration
	logger   *slog.Logger

	// timingHash is verified when a login names an unknown e-mail.
	timingHash string
}

// Option customizes a Handler.
type Option func(*Handler)

// WithClock replaces the time source used for token and account timestamps. A
// nil function is ignored.
func WithClock(now func() time.Time) Option {
	return func(h *Handler) {
		if now != nil {
			h.now = now
		}
	}
}

// WithTokenTTL sets the lifetime of issued access tokens. Non-positive values
// are ignored.
func WithTokenTTL(d time.Duration) Option {
	return func(h *Handler) {
		if d > 0 {
			h.tokenTTL = d
		}
	}
}

// WithIDGenerator replaces the generator of user and token identifiers. A nil
// function is ignored.
func WithIDGenerator(f func() (string, error)) Option {
	return func(h *Handler) {
		if f != nil {
			h.newID = f
		}
	}
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

// New builds a Handler. It fails if a dependency is missing or if the password
// hasher cannot produce the hash used to even out login timing.
func New(deps Dependencies, opts ...Option) (*Handler, error) {
	if deps.Users == nil || deps.Hasher == nil || deps.Tokens == nil {
		return nil, errors.New("httpapi: users, hasher and tokens are all required")
	}
	h := &Handler{
		users:    deps.Users,
		hasher:   deps.Hasher,
		tokens:   deps.Tokens,
		now:      time.Now,
		newID:    users.NewID,
		tokenTTL: DefaultTokenTTL,
		logger:   slog.New(slog.DiscardHandler),
	}
	for _, opt := range opts {
		opt(h)
	}

	hash, err := h.hasher.Hash(timingPassword)
	if err != nil {
		return nil, fmt.Errorf("httpapi: prepare timing hash: %w", err)
	}
	h.timingHash = hash
	return h, nil
}

// route couples a ServeMux pattern with its handler method. Keeping both in one
// table guarantees that Routes and Register can never disagree.
type route struct {
	pattern   string
	protected bool
	handle    func(*Handler, http.ResponseWriter, *http.Request)
}

var routes = []route{
	{"POST /auth/register", false, (*Handler).register},
	{"POST /auth/login", false, (*Handler).login},
	{"GET /me", true, (*Handler).me},
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

// Register mounts every route of the API on mux. Protected routes are wrapped
// in the bearer-token middleware.
func (h *Handler) Register(mux *http.ServeMux) {
	requireAuth := middleware.RequireAuth(h.tokens)
	for _, r := range routes {
		var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			r.handle(h, w, req)
		})
		if r.protected {
			handler = requireAuth(handler)
		}
		mux.Handle(r.pattern, handler)
	}
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if !h.decodeJSON(w, r, &req) {
		return
	}

	email := users.NormalizeEmail(req.Email)
	name := strings.TrimSpace(req.Name)
	if code, msg := validateRegistration(email, req.Password, name); code != "" {
		h.writeError(w, http.StatusUnprocessableEntity, code, msg)
		return
	}

	hash, err := h.hasher.Hash(req.Password)
	if err != nil {
		h.internalError(w, r, "hash password", err)
		return
	}
	id, err := h.newID()
	if err != nil {
		h.internalError(w, r, "generate user id", err)
		return
	}

	user := users.User{ID: id, Email: email, Name: name, PasswordHash: hash, CreatedAt: h.now().UTC()}
	switch err := h.users.Create(r.Context(), user); {
	case err == nil:
		h.writeJSON(w, http.StatusCreated, toUserResponse(user))
	case errors.Is(err, users.ErrEmailTaken):
		h.writeError(w, http.StatusConflict, "email_taken", "an account with this email already exists")
	default:
		h.internalError(w, r, "create user", err)
	}
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if !h.decodeJSON(w, r, &req) {
		return
	}
	if req.Email == "" || req.Password == "" {
		h.writeError(w, http.StatusUnprocessableEntity, "missing_credentials", "email and password are required")
		return
	}

	user, err := h.users.FindByEmail(r.Context(), req.Email)
	switch {
	case errors.Is(err, users.ErrNotFound):
		// Spend the same effort as for a known user so response times do not
		// reveal which e-mails are registered.
		_, _ = h.hasher.Verify(req.Password, h.timingHash)
		h.invalidCredentials(w)
		return
	case err != nil:
		h.internalError(w, r, "find user", err)
		return
	}

	ok, err := h.hasher.Verify(req.Password, user.PasswordHash)
	if err != nil {
		h.internalError(w, r, "verify password", err)
		return
	}
	if !ok {
		h.invalidCredentials(w)
		return
	}

	tokenID, err := h.newID()
	if err != nil {
		h.internalError(w, r, "generate token id", err)
		return
	}
	now := h.now().UTC()
	token, err := h.tokens.Sign(jwt.Claims{
		Subject:   user.ID,
		ID:        tokenID,
		IssuedAt:  now,
		ExpiresAt: now.Add(h.tokenTTL),
	})
	if err != nil {
		h.internalError(w, r, "sign token", err)
		return
	}

	w.Header().Set("Pragma", "no-cache")
	h.writeJSON(w, http.StatusOK, tokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int64(h.tokenTTL / time.Second),
	})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := middleware.ClaimsFromContext(r.Context())
	if !ok {
		// Only reachable if the route is mounted without the middleware.
		h.internalError(w, r, "read claims", errors.New("no claims in the request context"))
		return
	}

	user, err := h.users.FindByID(r.Context(), claims.Subject)
	switch {
	case errors.Is(err, users.ErrNotFound):
		// A well-formed token whose user no longer exists.
		w.Header().Set("WWW-Authenticate", `Bearer realm="api", error="invalid_token"`)
		h.writeError(w, http.StatusUnauthorized, "invalid_token", "token is invalid")
	case err != nil:
		h.internalError(w, r, "find user", err)
	default:
		h.writeJSON(w, http.StatusOK, toUserResponse(user))
	}
}

func (h *Handler) invalidCredentials(w http.ResponseWriter) {
	h.writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
}

func toUserResponse(u users.User) userResponse {
	return userResponse{ID: u.ID, Email: u.Email, Name: u.Name, CreatedAt: u.CreatedAt}
}

// validateRegistration returns an error code and message for the first invalid
// field, or empty strings when everything is acceptable. The e-mail must
// already be normalized and the name trimmed.
func validateRegistration(email, password, name string) (code, message string) {
	if !validEmail(email) {
		return "invalid_email", "email must be a valid address of up to 254 characters"
	}
	if n := utf8.RuneCountInString(password); n < minPasswordChars || n > maxPasswordChars {
		return "invalid_password", fmt.Sprintf("password must have between %d and %d characters", minPasswordChars, maxPasswordChars)
	}
	if utf8.RuneCountInString(name) > maxNameChars || strings.ContainsFunc(name, unicode.IsControl) {
		return "invalid_name", fmt.Sprintf("name must have at most %d characters and no control characters", maxNameChars)
	}
	return "", ""
}

// validEmail accepts a bare address (no display name, no angle brackets) with a
// dotted domain.
func validEmail(email string) bool {
	if email == "" || len(email) > maxEmailLength {
		return false
	}
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return false
	}
	_, domain, ok := strings.Cut(email, "@")
	return ok && strings.Contains(domain, ".") && !strings.HasSuffix(domain, ".")
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
		h.writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "request body must not exceed 4 KiB")
	case errors.Is(err, io.EOF):
		h.writeError(w, http.StatusBadRequest, "invalid_json", "request body must not be empty")
	case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
		h.writeError(w, http.StatusBadRequest, "invalid_json", "request body is not valid JSON")
	case errors.As(err, &typeErr) && typeErr.Field == "":
		h.writeError(w, http.StatusBadRequest, "invalid_json", "request body must be a JSON object")
	case errors.As(err, &typeErr):
		h.writeError(w, http.StatusBadRequest, "invalid_json", `field "`+typeErr.Field+`" has the wrong type`)
	default:
		// Unknown fields and any other decoding problem.
		h.writeError(w, http.StatusBadRequest, "invalid_json", strings.TrimPrefix(err.Error(), "json: "))
	}
}

// internalError logs the cause and answers with a generic 500.
func (h *Handler) internalError(w http.ResponseWriter, r *http.Request, what string, err error) {
	h.logger.Error("request failed", "method", r.Method, "path", r.URL.Path, "step", what, "error", err)
	h.writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}

func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	h.writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}

// writeJSON encodes v before touching the response so that an encoding failure
// can still produce a clean 500. Responses are never cacheable: they carry
// credentials or personal data.
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
