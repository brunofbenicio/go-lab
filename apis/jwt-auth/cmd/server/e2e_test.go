package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeClock is a manually advanced clock shared by every component under test.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// apiClient drives the server over real HTTP.
type apiClient struct {
	t    *testing.T
	base string
	http *http.Client
}

func newAPIClient(t *testing.T, base string) *apiClient {
	t.Helper()
	tr := &http.Transport{DisableKeepAlives: true}
	t.Cleanup(tr.CloseIdleConnections)
	return &apiClient{t: t, base: base, http: &http.Client{Transport: tr, Timeout: 10 * time.Second}}
}

type response struct {
	status int
	header http.Header
	body   []byte
}

func (r response) json(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.body, v); err != nil {
		t.Fatalf("response is not valid JSON: %v\nbody: %s", err, r.body)
	}
}

func (r response) errorCode(t *testing.T) string {
	t.Helper()
	var e struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	r.json(t, &e)
	if e.Error.Message == "" {
		t.Errorf("error response without a message: %s", r.body)
	}
	return e.Error.Code
}

func (c *apiClient) do(method, path, contentType, body string, headers ...string) response {
	c.t.Helper()
	req, err := http.NewRequest(method, c.base+path, strings.NewReader(body))
	if err != nil {
		c.t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatal(err)
	}
	return response{status: resp.StatusCode, header: resp.Header, body: data}
}

func (c *apiClient) postJSON(path, body string) response {
	c.t.Helper()
	return c.do(http.MethodPost, path, "application/json", body)
}

func (c *apiClient) me(authorization string) response {
	c.t.Helper()
	if authorization == "" {
		return c.do(http.MethodGet, "/me", "", "")
	}
	return c.do(http.MethodGet, "/me", "", "", "Authorization", authorization)
}

func b64(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

// forgeHS256 builds a correctly signed token for an arbitrary secret, the way
// an attacker who guessed (or does not know) the secret would.
func forgeHS256(secret, payload string) string {
	input := b64(`{"alg":"HS256","typ":"JWT"}`) + "." + b64(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(input))
	return input + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// TestEndToEnd walks through the full authentication flow over HTTP:
// registration, login, access to a protected route, attacks on the token, token
// expiry and the error responses.
func TestEndToEnd(t *testing.T) {
	clock := &fakeClock{t: time.Date(2025, time.March, 1, 12, 0, 0, 0, time.UTC)}
	var idMu sync.Mutex
	nextID := 0
	cfg, err := configFromEnv(envFrom(map[string]string{
		"JWT_SECRET": testSecret, "JWT_ISSUER": "auth.example.com", "TOKEN_TTL": "15m",
	}))
	if err != nil {
		t.Fatal(err)
	}
	handler, err := newHandler(cfg, discardLogger(), deps{
		now: clock.Now,
		newID: func() (string, error) {
			idMu.Lock()
			defer idMu.Unlock()
			nextID++
			return fmt.Sprintf("id-%d", nextID), nil
		},
		hasherOpts: fastDeps().hasherOpts,
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	api := newAPIClient(t, srv.URL)

	const (
		email    = "alice@example.com"
		pw       = "correct horse battery staple"
		userID   = "id-1"
		loginReq = `{"email":"alice@example.com","password":"correct horse battery staple"}`
	)

	t.Run("documentation is served", func(t *testing.T) {
		spec := api.do(http.MethodGet, "/openapi.yaml", "", "")
		if spec.status != http.StatusOK || !strings.HasPrefix(string(spec.body), "openapi: 3.0.3") {
			t.Errorf("GET /openapi.yaml = %d", spec.status)
		}
		page := api.do(http.MethodGet, "/docs", "", "")
		if page.status != http.StatusOK || !strings.Contains(string(page.body), "swagger-ui-dist@5.17.14") {
			t.Errorf("GET /docs = %d, Swagger UI reference missing", page.status)
		}
	})

	t.Run("the protected route refuses anonymous requests", func(t *testing.T) {
		resp := api.me("")
		if resp.status != http.StatusUnauthorized || resp.errorCode(t) != "missing_token" {
			t.Errorf("GET /me = %d %s", resp.status, resp.body)
		}
		if got := resp.header.Get("WWW-Authenticate"); got != `Bearer realm="api"` {
			t.Errorf("WWW-Authenticate = %q", got)
		}
	})

	t.Run("register", func(t *testing.T) {
		resp := api.postJSON("/auth/register", `{"email":"Alice@Example.com","password":"`+pw+`","name":"Alice Liddell"}`)
		if resp.status != http.StatusCreated {
			t.Fatalf("status = %d, want 201\n%s", resp.status, resp.body)
		}
		var user struct {
			ID        string `json:"id"`
			Email     string `json:"email"`
			Name      string `json:"name"`
			CreatedAt string `json:"created_at"`
		}
		resp.json(t, &user)
		if user.ID != userID || user.Email != email || user.Name != "Alice Liddell" || user.CreatedAt != "2025-03-01T12:00:00Z" {
			t.Errorf("user = %+v", user)
		}
		if strings.Contains(string(resp.body), pw) || strings.Contains(string(resp.body), "pbkdf2") {
			t.Errorf("the response leaks credentials: %s", resp.body)
		}
	})

	t.Run("registering the same e-mail again conflicts", func(t *testing.T) {
		resp := api.postJSON("/auth/register", `{"email":"ALICE@example.com","password":"another password"}`)
		if resp.status != http.StatusConflict || resp.errorCode(t) != "email_taken" {
			t.Errorf("status = %d, body = %s", resp.status, resp.body)
		}
	})

	t.Run("bad credentials look identical for known and unknown e-mails", func(t *testing.T) {
		wrong := api.postJSON("/auth/login", `{"email":"alice@example.com","password":"not the password"}`)
		unknown := api.postJSON("/auth/login", `{"email":"mallory@example.com","password":"not the password"}`)
		for name, resp := range map[string]response{"wrong password": wrong, "unknown e-mail": unknown} {
			if resp.status != http.StatusUnauthorized || resp.errorCode(t) != "invalid_credentials" {
				t.Errorf("%s: status = %d, body = %s", name, resp.status, resp.body)
			}
		}
		if string(wrong.body) != string(unknown.body) {
			t.Errorf("responses differ:\n%s\n%s", wrong.body, unknown.body)
		}
	})

	var token string
	t.Run("login issues an HS256 token", func(t *testing.T) {
		resp := api.postJSON("/auth/login", loginReq)
		if resp.status != http.StatusOK {
			t.Fatalf("status = %d, want 200\n%s", resp.status, resp.body)
		}
		var body struct {
			AccessToken string `json:"access_token"`
			TokenType   string `json:"token_type"`
			ExpiresIn   int    `json:"expires_in"`
		}
		resp.json(t, &body)
		if body.TokenType != "Bearer" || body.ExpiresIn != 900 {
			t.Errorf("body = %+v, want Bearer with expires_in 900", body)
		}
		token = body.AccessToken

		// Decode the token without the jwt package to check the wire format.
		parts := strings.Split(token, ".")
		if len(parts) != 3 {
			t.Fatalf("token has %d segments, want 3", len(parts))
		}
		header, err := base64.RawURLEncoding.DecodeString(parts[0])
		if err != nil {
			t.Fatal(err)
		}
		payload, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			t.Fatal(err)
		}
		if string(header) != `{"alg":"HS256","typ":"JWT"}` {
			t.Errorf("header = %s", header)
		}
		var claims map[string]any
		if err := json.Unmarshal(payload, &claims); err != nil {
			t.Fatal(err)
		}
		iat, exp := int64(claims["iat"].(float64)), int64(claims["exp"].(float64))
		if claims["sub"] != userID || claims["iss"] != "auth.example.com" || exp-iat != 900 {
			t.Errorf("claims = %v", claims)
		}
		if jti, _ := claims["jti"].(string); jti == "" || jti == userID {
			t.Errorf("jti = %q, want a unique token ID", jti)
		}
		if iat != clock.Now().Unix() {
			t.Errorf("iat = %d, want %d", iat, clock.Now().Unix())
		}
	})

	t.Run("the token opens the protected route", func(t *testing.T) {
		resp := api.me("Bearer " + token)
		if resp.status != http.StatusOK {
			t.Fatalf("status = %d, want 200\n%s", resp.status, resp.body)
		}
		var user struct {
			ID    string `json:"id"`
			Email string `json:"email"`
			Name  string `json:"name"`
		}
		resp.json(t, &user)
		if user.ID != userID || user.Email != email || user.Name != "Alice Liddell" {
			t.Errorf("user = %+v", user)
		}
	})

	t.Run("forged and tampered tokens are refused", func(t *testing.T) {
		parts := strings.Split(token, ".")
		far := fmt.Sprintf(`{"sub":%q,"iss":"auth.example.com","exp":4102444800}`, userID)

		tests := []struct {
			name  string
			token string
		}{
			{"alg none, no signature", b64(`{"alg":"none","typ":"JWT"}`) + "." + b64(far) + "."},
			{"alg none, signature kept", b64(`{"alg":"none","typ":"JWT"}`) + "." + b64(far) + "." + parts[2]},
			{"alg None", b64(`{"alg":"None"}`) + "." + b64(far) + "."},
			{"payload swapped, signature kept", parts[0] + "." + b64(far) + "." + parts[2]},
			{"signed with another secret", forgeHS256("a-different-fake-secret-of-32-bytes!!", far)},
			{"signed with a guessable secret", forgeHS256("secret", far)},
			{"signature stripped", parts[0] + "." + parts[1] + "."},
			{"not a JWT", "hello"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				resp := api.me("Bearer " + tt.token)
				if resp.status != http.StatusUnauthorized {
					t.Fatalf("status = %d, want 401\n%s", resp.status, resp.body)
				}
				if code := resp.errorCode(t); code != "invalid_token" {
					t.Errorf("error code = %q, want invalid_token", code)
				}
			})
		}
	})

	t.Run("the token works until it expires", func(t *testing.T) {
		clock.Advance(14*time.Minute + 59*time.Second)
		if resp := api.me("Bearer " + token); resp.status != http.StatusOK {
			t.Fatalf("one second before expiry: status = %d\n%s", resp.status, resp.body)
		}

		clock.Advance(time.Second)
		resp := api.me("Bearer " + token)
		if resp.status != http.StatusUnauthorized || resp.errorCode(t) != "token_expired" {
			t.Fatalf("at expiry: status = %d, body = %s", resp.status, resp.body)
		}
		if got := resp.header.Get("WWW-Authenticate"); !strings.Contains(got, `error="invalid_token"`) {
			t.Errorf("WWW-Authenticate = %q, want an invalid_token challenge", got)
		}
	})

	t.Run("logging in again yields a fresh working token", func(t *testing.T) {
		var body struct {
			AccessToken string `json:"access_token"`
		}
		api.postJSON("/auth/login", loginReq).json(t, &body)
		if body.AccessToken == "" || body.AccessToken == token {
			t.Fatal("expected a new token")
		}
		if resp := api.me("Bearer " + body.AccessToken); resp.status != http.StatusOK {
			t.Errorf("status = %d, want 200", resp.status)
		}
	})

	t.Run("error responses", func(t *testing.T) {
		tests := []struct {
			name        string
			path        string
			contentType string
			body        string
			wantStatus  int
			wantCode    string
		}{
			{"register: invalid e-mail", "/auth/register", "application/json", `{"email":"nope","password":"12345678"}`, 422, "invalid_email"},
			{"register: short password", "/auth/register", "application/json", `{"email":"b@example.com","password":"1234567"}`, 422, "invalid_password"},
			{"register: malformed JSON", "/auth/register", "application/json", `{`, 400, "invalid_json"},
			{"register: unknown field", "/auth/register", "application/json", `{"email":"b@example.com","password":"12345678","admin":true}`, 400, "invalid_json"},
			{"register: wrong content type", "/auth/register", "text/plain", `{}`, 415, "unsupported_media_type"},
			{"login: missing fields", "/auth/login", "application/json", `{}`, 422, "missing_credentials"},
			{"login: malformed JSON", "/auth/login", "application/json", `nope`, 400, "invalid_json"},
			{"login: wrong content type", "/auth/login", "", loginReq, 415, "unsupported_media_type"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				resp := api.do(http.MethodPost, tt.path, tt.contentType, tt.body)
				if resp.status != tt.wantStatus || resp.errorCode(t) != tt.wantCode {
					t.Errorf("status = %d, body = %s; want %d %s", resp.status, resp.body, tt.wantStatus, tt.wantCode)
				}
			})
		}
	})

	t.Run("wrong methods are refused", func(t *testing.T) {
		for _, tt := range []struct{ method, path string }{
			{"GET", "/auth/login"},
			{"PUT", "/auth/register"},
			{"POST", "/me"},
		} {
			if resp := api.do(tt.method, tt.path, "", ""); resp.status != http.StatusMethodNotAllowed {
				t.Errorf("%s %s = %d, want 405", tt.method, tt.path, resp.status)
			}
		}
	})
}
