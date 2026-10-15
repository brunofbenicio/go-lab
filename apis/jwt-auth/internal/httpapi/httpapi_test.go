package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brunofbenicio/go-lab/apis/jwt-auth/internal/jwt"
	"github.com/brunofbenicio/go-lab/apis/jwt-auth/internal/password"
	"github.com/brunofbenicio/go-lab/apis/jwt-auth/internal/users"
)

var (
	t0 = time.Date(2025, time.March, 1, 12, 0, 0, 0, time.UTC)
	// testSecret is an obviously fake key used only by these tests.
	testSecret = []byte("test-only-secret-do-not-use-in-production-0123456789")
)

// clock is a manually advanced time source.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// countingHasher wraps a PasswordHasher and counts Verify calls.
type countingHasher struct {
	PasswordHasher
	mu       sync.Mutex
	verifies int
}

func (c *countingHasher) Verify(pw, encoded string) (bool, error) {
	c.mu.Lock()
	c.verifies++
	c.mu.Unlock()
	return c.PasswordHasher.Verify(pw, encoded)
}

// env is a fully wired handler on top of real collaborators.
type env struct {
	mux    *http.ServeMux
	users  *users.Memory
	clock  *clock
	signer *jwt.HS256
	hasher *countingHasher
}

// sequentialIDs returns a generator yielding "id-1", "id-2", ...
func sequentialIDs() func() (string, error) {
	var mu sync.Mutex
	n := 0
	return func() (string, error) {
		mu.Lock()
		defer mu.Unlock()
		n++
		return fmt.Sprintf("id-%d", n), nil
	}
}

func newEnv(t *testing.T, opts ...Option) *env {
	t.Helper()
	clk := &clock{t: t0}
	signer, err := jwt.NewHS256(testSecret, jwt.WithClock(clk.Now), jwt.WithIssuer("auth.example.com"))
	if err != nil {
		t.Fatal(err)
	}
	realHasher, err := password.NewHasher(password.WithIterations(10))
	if err != nil {
		t.Fatal(err)
	}
	hasher := &countingHasher{PasswordHasher: realHasher}
	store := users.NewMemory()

	opts = append([]Option{WithClock(clk.Now), WithIDGenerator(sequentialIDs())}, opts...)
	h, err := New(Dependencies{Users: store, Hasher: hasher, Tokens: signer}, opts...)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	mux := http.NewServeMux()
	h.Register(mux)
	return &env{mux: mux, users: store, clock: clk, signer: signer, hasher: hasher}
}

func (e *env) do(t *testing.T, method, target, contentType, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	return serve(t, e.mux, method, target, contentType, body, headers...)
}

func serve(t *testing.T, h http.Handler, method, target, contentType, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func (e *env) postJSON(t *testing.T, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return e.do(t, http.MethodPost, path, "application/json", body)
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("response body is not valid JSON: %v\nbody: %s", err, rec.Body.String())
	}
	return v
}

func assertError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d\nbody: %s", rec.Code, wantStatus, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	body := decode[errorBody](t, rec)
	if body.Error.Code != wantCode {
		t.Errorf("error code = %q, want %q (message %q)", body.Error.Code, wantCode, body.Error.Message)
	}
	if body.Error.Message == "" {
		t.Error("error message is empty")
	}
}

// register creates the account used by most tests and returns its ID.
func (e *env) register(t *testing.T, email, pw string) string {
	t.Helper()
	rec := e.postJSON(t, "/auth/register", fmt.Sprintf(`{"email":%q,"password":%q,"name":"Alice"}`, email, pw))
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: status = %d\n%s", rec.Code, rec.Body.String())
	}
	return decode[userResponse](t, rec).ID
}

// login returns the access token for the credentials.
func (e *env) login(t *testing.T, email, pw string) string {
	t.Helper()
	rec := e.postJSON(t, "/auth/login", fmt.Sprintf(`{"email":%q,"password":%q}`, email, pw))
	if rec.Code != http.StatusOK {
		t.Fatalf("login: status = %d\n%s", rec.Code, rec.Body.String())
	}
	return decode[tokenResponse](t, rec).AccessToken
}

func TestRoutesAreRegistered(t *testing.T) {
	e := newEnv(t)
	for _, pattern := range Routes() {
		method, path, ok := strings.Cut(pattern, " ")
		if !ok {
			t.Fatalf("malformed pattern %q", pattern)
		}
		req := httptest.NewRequest(method, path, nil)
		if _, got := e.mux.Handler(req); got != pattern {
			t.Errorf("%s %s matched pattern %q, want %q", method, path, got, pattern)
		}
	}
}

func TestNewValidatesDependencies(t *testing.T) {
	hasher, err := password.NewHasher(password.WithIterations(10))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := jwt.NewHS256(testSecret)
	if err != nil {
		t.Fatal(err)
	}
	store := users.NewMemory()

	tests := []struct {
		name string
		deps Dependencies
	}{
		{"missing store", Dependencies{Hasher: hasher, Tokens: signer}},
		{"missing hasher", Dependencies{Users: store, Tokens: signer}},
		{"missing tokens", Dependencies{Users: store, Hasher: hasher}},
		{"failing hasher", Dependencies{Users: store, Hasher: failingHasher{}, Tokens: signer}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.deps); err == nil {
				t.Error("New() succeeded with unusable dependencies")
			}
		})
	}
}

func TestRegister(t *testing.T) {
	e := newEnv(t)
	rec := e.postJSON(t, "/auth/register",
		`{"email":"  Alice@Example.COM ","password":"correct horse battery","name":"  Alice Liddell  "}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201\n%s", rec.Code, rec.Body.String())
	}
	body := decode[userResponse](t, rec)
	if body.ID != "id-1" || body.Email != "alice@example.com" || body.Name != "Alice Liddell" {
		t.Errorf("body = %+v", body)
	}
	if !body.CreatedAt.Equal(t0) {
		t.Errorf("created_at = %v, want %v", body.CreatedAt, t0)
	}
	for _, leaked := range []string{"password", "hash", "pbkdf2", "correct horse"} {
		if strings.Contains(strings.ToLower(rec.Body.String()), leaked) {
			t.Errorf("response mentions %q: %s", leaked, rec.Body.String())
		}
	}

	stored, err := e.users.FindByID(context.Background(), "id-1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stored.PasswordHash, "correct horse") || !strings.HasPrefix(stored.PasswordHash, "$pbkdf2-sha256$") {
		t.Errorf("stored hash = %q, want a PBKDF2 hash that does not contain the password", stored.PasswordHash)
	}
}

func TestRegisterWithoutName(t *testing.T) {
	e := newEnv(t)
	rec := e.postJSON(t, "/auth/register", `{"email":"bob@example.com","password":"12345678"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201\n%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"name"`) {
		t.Errorf("empty name should be omitted: %s", rec.Body.String())
	}
}

func TestRegisterValidation(t *testing.T) {
	long := strings.Repeat("a", 64)
	tests := []struct {
		name     string
		email    string
		password string
		display  string
		wantCode string // empty means the registration must succeed
	}{
		{"valid", "alice@example.com", "12345678", "", ""},
		{"valid subdomain and plus tag", "alice+tag@mail.example.co.uk", "12345678", "", ""},
		{"password of exactly 8 characters", "a1@example.com", "12345678", "", ""},
		{"password of 8 emoji counts characters, not bytes", "a2@example.com", "😀😀😀😀😀😀😀😀", "", ""},
		{"password of exactly 128 characters", "a3@example.com", strings.Repeat("x", 128), "", ""},
		{"name of exactly 100 characters", "a4@example.com", "12345678", strings.Repeat("é", 100), ""},

		{"empty email", "", "12345678", "", "invalid_email"},
		{"email without at sign", "alice.example.com", "12345678", "", "invalid_email"},
		{"email without domain", "alice@", "12345678", "", "invalid_email"},
		{"email without local part", "@example.com", "12345678", "", "invalid_email"},
		{"email domain without dot", "alice@localhost", "12345678", "", "invalid_email"},
		{"email domain ending in a dot", "alice@example.com.", "12345678", "", "invalid_email"},
		{"email with display name", "Alice <alice@example.com>", "12345678", "", "invalid_email"},
		{"email with inner space", "ali ce@example.com", "12345678", "", "invalid_email"},
		{"two addresses", "a@example.com, b@example.com", "12345678", "", "invalid_email"},
		{"email longer than 254 characters", long + "@" + long + "." + long + "." + long + ".example.com", "12345678", "", "invalid_email"},

		{"empty password", "b1@example.com", "", "", "invalid_password"},
		{"password of 7 characters", "b2@example.com", "1234567", "", "invalid_password"},
		{"password of 7 emoji", "b3@example.com", "😀😀😀😀😀😀😀", "", "invalid_password"},
		{"password of 129 characters", "b4@example.com", strings.Repeat("x", 129), "", "invalid_password"},

		{"name of 101 characters", "c1@example.com", "12345678", strings.Repeat("é", 101), "invalid_name"},
		{"name with a control character", "c2@example.com", "12345678", "Al\x00ice", "invalid_name"},
		{"name with a newline", "c3@example.com", "12345678", "Al\nice", "invalid_name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			body, err := json.Marshal(registerRequest{Email: tt.email, Password: tt.password, Name: tt.display})
			if err != nil {
				t.Fatal(err)
			}
			rec := e.postJSON(t, "/auth/register", string(body))
			if tt.wantCode == "" {
				if rec.Code != http.StatusCreated {
					t.Errorf("status = %d, want 201\n%s", rec.Code, rec.Body.String())
				}
				return
			}
			assertError(t, rec, http.StatusUnprocessableEntity, tt.wantCode)
		})
	}
}

func TestRegisterRejectsBadRequests(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
		wantCode    string
	}{
		{"missing content type", "", `{"email":"a@example.com","password":"12345678"}`, 415, "unsupported_media_type"},
		{"form content type", "application/x-www-form-urlencoded", `email=a@example.com`, 415, "unsupported_media_type"},
		{"empty body", "application/json", ``, 400, "invalid_json"},
		{"truncated JSON", "application/json", `{"email":"a@exa`, 400, "invalid_json"},
		{"not JSON", "application/json", `email=a@example.com`, 400, "invalid_json"},
		{"array instead of object", "application/json", `[]`, 400, "invalid_json"},
		{"unknown field", "application/json", `{"email":"a@example.com","password":"12345678","admin":true}`, 400, "invalid_json"},
		{"wrong field type", "application/json", `{"email":42,"password":"12345678"}`, 400, "invalid_json"},
		{"trailing data", "application/json", `{"email":"a@example.com","password":"12345678"} {}`, 400, "invalid_json"},
		{"body too large", "application/json", `{"email":"a@example.com","password":"` + strings.Repeat("x", MaxBodyBytes) + `"}`, 413, "payload_too_large"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			rec := e.do(t, http.MethodPost, "/auth/register", tt.contentType, tt.body)
			assertError(t, rec, tt.wantStatus, tt.wantCode)
		})
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	e := newEnv(t)
	e.register(t, "alice@example.com", "12345678")

	for _, email := range []string{"alice@example.com", "ALICE@EXAMPLE.COM", " alice@example.com "} {
		rec := e.postJSON(t, "/auth/register", fmt.Sprintf(`{"email":%q,"password":"another-password"}`, email))
		assertError(t, rec, http.StatusConflict, "email_taken")
	}
}

func TestLogin(t *testing.T) {
	e := newEnv(t)
	id := e.register(t, "alice@example.com", "correct horse battery")

	rec := e.postJSON(t, "/auth/login", `{"email":"alice@example.com","password":"correct horse battery"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	if p := rec.Header().Get("Pragma"); p != "no-cache" {
		t.Errorf("Pragma = %q, want no-cache", p)
	}

	body := decode[tokenResponse](t, rec)
	if body.TokenType != "Bearer" || body.ExpiresIn != 900 {
		t.Errorf("body = %+v, want Bearer with expires_in 900", body)
	}

	claims, err := e.signer.Verify(body.AccessToken)
	if err != nil {
		t.Fatalf("issued token does not verify: %v", err)
	}
	if claims.Subject != id {
		t.Errorf("sub = %q, want %q", claims.Subject, id)
	}
	if claims.ID != "id-2" {
		t.Errorf("jti = %q, want id-2", claims.ID)
	}
	if !claims.IssuedAt.Equal(t0) || !claims.ExpiresAt.Equal(t0.Add(15*time.Minute)) {
		t.Errorf("iat = %v, exp = %v; want %v and %v", claims.IssuedAt, claims.ExpiresAt, t0, t0.Add(15*time.Minute))
	}
	if claims.Issuer != "auth.example.com" {
		t.Errorf("iss = %q, want the configured issuer", claims.Issuer)
	}
}

func TestLoginEmailIsCaseInsensitive(t *testing.T) {
	e := newEnv(t)
	e.register(t, "alice@example.com", "correct horse battery")
	if token := e.login(t, "ALICE@Example.com", "correct horse battery"); token == "" {
		t.Error("empty token")
	}
}

func TestLoginCustomTokenTTL(t *testing.T) {
	e := newEnv(t, WithTokenTTL(2*time.Hour), WithTokenTTL(0), WithTokenTTL(-time.Second))
	e.register(t, "alice@example.com", "12345678")
	rec := e.postJSON(t, "/auth/login", `{"email":"alice@example.com","password":"12345678"}`)
	if got := decode[tokenResponse](t, rec).ExpiresIn; got != 7200 {
		t.Errorf("expires_in = %d, want 7200 (invalid TTL options must be ignored)", got)
	}
}

func TestLoginFailures(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{"wrong password", `{"email":"alice@example.com","password":"wrong-password"}`, 401, "invalid_credentials"},
		{"password differing in case", `{"email":"alice@example.com","password":"CORRECT HORSE BATTERY"}`, 401, "invalid_credentials"},
		{"unknown email", `{"email":"nobody@example.com","password":"correct horse battery"}`, 401, "invalid_credentials"},
		{"missing password", `{"email":"alice@example.com"}`, 422, "missing_credentials"},
		{"missing email", `{"password":"correct horse battery"}`, 422, "missing_credentials"},
		{"empty object", `{}`, 422, "missing_credentials"},
		{"malformed JSON", `{"email":`, 400, "invalid_json"},
		{"unknown field", `{"email":"alice@example.com","password":"x","remember":true}`, 400, "invalid_json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			e.register(t, "alice@example.com", "correct horse battery")
			assertError(t, e.postJSON(t, "/auth/login", tt.body), tt.wantStatus, tt.wantCode)
		})
	}
}

func TestLoginDoesNotRevealWhetherTheEmailExists(t *testing.T) {
	e := newEnv(t)
	e.register(t, "alice@example.com", "correct horse battery")

	wrongPassword := e.postJSON(t, "/auth/login", `{"email":"alice@example.com","password":"wrong-password"}`)
	unknownEmail := e.postJSON(t, "/auth/login", `{"email":"nobody@example.com","password":"wrong-password"}`)

	if wrongPassword.Code != unknownEmail.Code || wrongPassword.Body.String() != unknownEmail.Body.String() {
		t.Errorf("responses differ:\n known email: %d %s\n unknown email: %d %s",
			wrongPassword.Code, wrongPassword.Body.String(), unknownEmail.Code, unknownEmail.Body.String())
	}
}

func TestLoginSpendsTheSameWorkForUnknownEmails(t *testing.T) {
	e := newEnv(t)
	e.register(t, "alice@example.com", "correct horse battery")

	before := e.hasher.verifies
	e.postJSON(t, "/auth/login", `{"email":"alice@example.com","password":"wrong-password"}`)
	knownCost := e.hasher.verifies - before

	before = e.hasher.verifies
	e.postJSON(t, "/auth/login", `{"email":"nobody@example.com","password":"wrong-password"}`)
	unknownCost := e.hasher.verifies - before

	if knownCost != 1 || unknownCost != 1 {
		t.Errorf("password verifications: known email %d, unknown email %d; want 1 each", knownCost, unknownCost)
	}
}

func TestMe(t *testing.T) {
	e := newEnv(t)
	id := e.register(t, "alice@example.com", "correct horse battery")
	token := e.login(t, "alice@example.com", "correct horse battery")

	rec := e.do(t, http.MethodGet, "/me", "", "", "Authorization", "Bearer "+token)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := decode[userResponse](t, rec)
	if body.ID != id || body.Email != "alice@example.com" || body.Name != "Alice" || !body.CreatedAt.Equal(t0) {
		t.Errorf("body = %+v", body)
	}
	if strings.Contains(strings.ToLower(rec.Body.String()), "hash") || strings.Contains(rec.Body.String(), "pbkdf2") {
		t.Errorf("response leaks the password hash: %s", rec.Body.String())
	}
}

func TestMeRejectsBadTokens(t *testing.T) {
	e := newEnv(t)
	e.register(t, "alice@example.com", "correct horse battery")
	token := e.login(t, "alice@example.com", "correct horse battery")

	otherSigner, err := jwt.NewHS256([]byte("another-fake-secret-for-tests-only-9876543210"), jwt.WithClock(e.clock.Now))
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := otherSigner.Sign(jwt.Claims{Subject: "id-1", ExpiresAt: t0.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		header   string
		advance  time.Duration
		wantCode string
	}{
		{"no header", "", 0, "missing_token"},
		{"basic auth", "Basic YWxpY2U6cGFzcw==", 0, "invalid_request"},
		{"garbage token", "Bearer not-a-jwt", 0, "invalid_token"},
		{"tampered token", "Bearer " + token[:len(token)-3] + "AAA", 0, "invalid_token"},
		{"signed with another secret", "Bearer " + foreign, 0, "invalid_token"},
		{"expired token", "Bearer " + token, 16 * time.Minute, "token_expired"},
		{"token right at expiry", "Bearer " + token, 15 * time.Minute, "token_expired"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e.clock.Advance(tt.advance)
			defer e.clock.Advance(-tt.advance)

			var rec *httptest.ResponseRecorder
			if tt.header == "" {
				rec = e.do(t, http.MethodGet, "/me", "", "")
			} else {
				rec = e.do(t, http.MethodGet, "/me", "", "", "Authorization", tt.header)
			}
			assertError(t, rec, http.StatusUnauthorized, tt.wantCode)
			if rec.Header().Get("WWW-Authenticate") == "" {
				t.Error("WWW-Authenticate header is missing")
			}
		})
	}
}

func TestMeWithUnknownSubject(t *testing.T) {
	e := newEnv(t)
	// A perfectly valid token whose user does not exist (for instance because
	// the account was deleted after the token was issued).
	token, err := e.signer.Sign(jwt.Claims{Subject: "ghost", ExpiresAt: t0.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	rec := e.do(t, http.MethodGet, "/me", "", "", "Authorization", "Bearer "+token)
	assertError(t, rec, http.StatusUnauthorized, "invalid_token")
	if rec.Header().Get("WWW-Authenticate") == "" {
		t.Error("WWW-Authenticate header is missing")
	}
}

func TestMeWithoutMiddlewareFailsClosed(t *testing.T) {
	e := newEnv(t)
	h, err := New(Dependencies{Users: e.users, Hasher: e.hasher, Tokens: e.signer})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.me(rec, httptest.NewRequest(http.MethodGet, "/me", nil))
	assertError(t, rec, http.StatusInternalServerError, "internal_error")
}

func TestMethodNotAllowed(t *testing.T) {
	e := newEnv(t)
	tests := []struct{ method, path, allow string }{
		{http.MethodGet, "/auth/register", "POST"},
		{http.MethodGet, "/auth/login", "POST"},
		{http.MethodPost, "/me", "GET"},
		{http.MethodDelete, "/me", "GET"},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			rec := e.do(t, tt.method, tt.path, "", "")
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405", rec.Code)
			}
			if allow := rec.Header().Get("Allow"); !strings.Contains(allow, tt.allow) {
				t.Errorf("Allow = %q, want it to contain %q", allow, tt.allow)
			}
		})
	}
}

// --- failure injection ------------------------------------------------------

type failingHasher struct{}

func (failingHasher) Hash(string) (string, error)         { return "", errors.New("hash failure") }
func (failingHasher) Verify(string, string) (bool, error) { return false, errors.New("verify failure") }

type failingTokens struct{ Tokens }

func (failingTokens) Sign(jwt.Claims) (string, error) { return "", errors.New("sign failure") }

// failingStore wraps a users.Store and fails the selected operations.
type failingStore struct {
	users.Store
	create, byEmail, byID error
}

func (f failingStore) Create(ctx context.Context, u users.User) error {
	if f.create != nil {
		return f.create
	}
	return f.Store.Create(ctx, u)
}

func (f failingStore) FindByEmail(ctx context.Context, email string) (users.User, error) {
	if f.byEmail != nil {
		return users.User{}, f.byEmail
	}
	return f.Store.FindByEmail(ctx, email)
}

func (f failingStore) FindByID(ctx context.Context, id string) (users.User, error) {
	if f.byID != nil {
		return users.User{}, f.byID
	}
	return f.Store.FindByID(ctx, id)
}

func TestInternalErrorsAreHiddenAndLogged(t *testing.T) {
	realHasher, err := password.NewHasher(password.WithIterations(10))
	if err != nil {
		t.Fatal(err)
	}
	signer, err := jwt.NewHS256(testSecret, jwt.WithClock((&clock{t: t0}).Now))
	if err != nil {
		t.Fatal(err)
	}
	// A user whose stored hash is corrupt, to exercise password.ErrInvalidHash.
	seed := users.NewMemory()
	good, err := realHasher.Hash("12345678")
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range []users.User{
		{ID: "u-ok", Email: "ok@example.com", PasswordHash: good, CreatedAt: t0},
		{ID: "u-bad", Email: "bad@example.com", PasswordHash: "corrupted", CreatedAt: t0},
	} {
		if err := seed.Create(context.Background(), u); err != nil {
			t.Fatal(err)
		}
	}
	okToken, err := signer.Sign(jwt.Claims{Subject: "u-ok", ExpiresAt: t0.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}

	const secret = "db-secret-host"
	boom := errors.New("connection to " + secret + " refused")
	registerBody := `{"email":"new@example.com","password":"12345678"}`
	loginOK := `{"email":"ok@example.com","password":"12345678"}`

	tests := []struct {
		name   string
		store  users.Store
		hasher PasswordHasher
		tokens Tokens
		ids    func() (string, error)
		method string
		path   string
		body   string
		auth   string
	}{
		{name: "register: store failure", store: failingStore{Store: seed, create: boom}, method: "POST", path: "/auth/register", body: registerBody},
		{name: "register: id generator failure", ids: func() (string, error) { return "", boom }, method: "POST", path: "/auth/register", body: registerBody},
		{name: "login: store failure", store: failingStore{Store: seed, byEmail: boom}, method: "POST", path: "/auth/login", body: loginOK},
		{name: "login: corrupt stored hash", method: "POST", path: "/auth/login", body: `{"email":"bad@example.com","password":"12345678"}`},
		{name: "login: token signing failure", tokens: failingTokens{signer}, method: "POST", path: "/auth/login", body: loginOK},
		{name: "login: id generator failure", ids: func() (string, error) { return "", boom }, method: "POST", path: "/auth/login", body: loginOK},
		{name: "me: store failure", store: failingStore{Store: seed, byID: boom}, method: "GET", path: "/me", auth: "Bearer " + okToken},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var store users.Store = seed
			if tt.store != nil {
				store = tt.store
			}
			var hasher PasswordHasher = realHasher
			if tt.hasher != nil {
				hasher = tt.hasher
			}
			var tokens Tokens = signer
			if tt.tokens != nil {
				tokens = tt.tokens
			}

			var logs bytes.Buffer
			opts := []Option{WithLogger(slog.New(slog.NewTextHandler(&logs, nil)))}
			if tt.ids != nil {
				opts = append(opts, WithIDGenerator(tt.ids))
			}
			h, err := New(Dependencies{Users: store, Hasher: hasher, Tokens: tokens}, opts...)
			if err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			h.Register(mux)

			var headers []string
			if tt.auth != "" {
				headers = []string{"Authorization", tt.auth}
			}
			contentType := ""
			if tt.body != "" {
				contentType = "application/json"
			}
			rec := serve(t, mux, tt.method, tt.path, contentType, tt.body, headers...)

			assertError(t, rec, http.StatusInternalServerError, "internal_error")
			if strings.Contains(rec.Body.String(), secret) {
				t.Errorf("internal details leaked to the client: %s", rec.Body.String())
			}
			if logs.Len() == 0 {
				t.Error("the failure was not logged")
			}
		})
	}
}

func TestOptionsIgnoreNilValues(t *testing.T) {
	e := newEnv(t, WithClock(nil), WithIDGenerator(nil), WithLogger(nil))
	e.register(t, "alice@example.com", "12345678")
}
