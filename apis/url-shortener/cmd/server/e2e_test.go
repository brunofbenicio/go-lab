package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brunofbenicio/go-lab/apis/url-shortener/internal/shortener"
)

// fakeClock is a manually advanced clock shared by the service under test.
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

// drawSequence returns a random source that yields the given values, one per
// code draw, as big-endian uint64s.
func drawSequence(values ...uint64) io.Reader {
	var buf bytes.Buffer
	for _, v := range values {
		_ = binary.Write(&buf, binary.BigEndian, v)
	}
	return &buf
}

// apiClient drives the server over real HTTP without following redirects.
type apiClient struct {
	t    *testing.T
	base string
	http *http.Client
}

func newAPIClient(t *testing.T, base string) *apiClient {
	t.Helper()
	tr := &http.Transport{DisableKeepAlives: true}
	t.Cleanup(tr.CloseIdleConnections)
	return &apiClient{
		t:    t,
		base: base,
		http: &http.Client{
			Transport: tr,
			Timeout:   10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// do sends a request and returns the status, headers and body.
func (c *apiClient) do(method, path, contentType, body string) (int, http.Header, []byte) {
	c.t.Helper()
	req, err := http.NewRequest(method, c.base+path, strings.NewReader(body))
	if err != nil {
		c.t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
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
	return resp.StatusCode, resp.Header, data
}

func (c *apiClient) createLink(body string) (int, createdLink, apiError) {
	c.t.Helper()
	status, _, data := c.do(http.MethodPost, "/api/links", "application/json", body)
	var link createdLink
	var apiErr apiError
	if status == http.StatusCreated {
		mustUnmarshal(c.t, data, &link)
	} else {
		mustUnmarshal(c.t, data, &apiErr)
	}
	return status, link, apiErr
}

func (c *apiClient) stats(code string) (int, linkStats, apiError) {
	c.t.Helper()
	status, _, data := c.do(http.MethodGet, "/api/links/"+code+"/stats", "", "")
	var stats linkStats
	var apiErr apiError
	if status == http.StatusOK {
		mustUnmarshal(c.t, data, &stats)
	} else {
		mustUnmarshal(c.t, data, &apiErr)
	}
	return status, stats, apiErr
}

type createdLink struct {
	Code      string `json:"code"`
	ShortURL  string `json:"short_url"`
	URL       string `json:"url"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at"`
}

type linkStats struct {
	Code         string `json:"code"`
	URL          string `json:"url"`
	CreatedAt    string `json:"created_at"`
	ExpiresAt    string `json:"expires_at"`
	Hits         int64  `json:"hits"`
	LastAccessAt string `json:"last_access_at"`
	Expired      bool   `json:"expired"`
}

type apiError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func mustUnmarshal(t *testing.T, data []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("response is not valid JSON: %v\nbody: %s", err, data)
	}
}

// TestEndToEnd walks through the whole life of two links over HTTP: creation
// (including a transparent code collision), redirects, statistics, expiry and
// the error responses.
func TestEndToEnd(t *testing.T) {
	clock := &fakeClock{t: time.Date(2025, time.March, 1, 12, 0, 0, 0, time.UTC)}
	// Draw 1 creates link A. Link B draws 1 again (a collision with A) and is
	// then given draw 2, so its code must be "0000002".
	handler, err := newHandler(config{Port: "0"}, discardLogger(),
		shortener.WithClock(clock.Now),
		shortener.WithRandom(drawSequence(1, 1, 2)),
	)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	api := newAPIClient(t, srv.URL)

	const (
		targetA = "https://example.com/articles/2025/a-very-long-article-title?utm_source=newsletter"
		targetB = "https://example.com/promo"
		codeA   = "0000001"
		codeB   = "0000002"
	)

	t.Run("documentation is served", func(t *testing.T) {
		status, header, body := api.do(http.MethodGet, "/openapi.yaml", "", "")
		if status != http.StatusOK || !strings.HasPrefix(string(body), "openapi: 3.0.3") {
			t.Errorf("GET /openapi.yaml = %d, body starts %.20q", status, body)
		}
		if ct := header.Get("Content-Type"); ct != "application/yaml" {
			t.Errorf("openapi.yaml Content-Type = %q", ct)
		}

		status, header, body = api.do(http.MethodGet, "/docs", "", "")
		if status != http.StatusOK || !strings.Contains(string(body), "swagger-ui-dist@5.17.14") {
			t.Errorf("GET /docs = %d, Swagger UI reference missing", status)
		}
		if ct := header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Errorf("docs Content-Type = %q", ct)
		}
	})

	t.Run("create a permanent link", func(t *testing.T) {
		status, link, _ := api.createLink(`{"url":"` + targetA + `"}`)
		if status != http.StatusCreated {
			t.Fatalf("status = %d, want 201", status)
		}
		if link.Code != codeA || link.URL != targetA {
			t.Errorf("link = %+v, want code %s for %s", link, codeA, targetA)
		}
		if want := srv.URL + "/" + codeA; link.ShortURL != want {
			t.Errorf("short_url = %q, want %q", link.ShortURL, want)
		}
		if link.CreatedAt != "2025-03-01T12:00:00Z" || link.ExpiresAt != "" {
			t.Errorf("created_at = %q, expires_at = %q", link.CreatedAt, link.ExpiresAt)
		}
	})

	t.Run("create a temporary link and survive a code collision", func(t *testing.T) {
		status, header, data := api.do(http.MethodPost, "/api/links", "application/json",
			`{"url":"`+targetB+`","ttl_seconds":3600}`)
		if status != http.StatusCreated {
			t.Fatalf("status = %d, want 201\nbody: %s", status, data)
		}
		var link createdLink
		mustUnmarshal(t, data, &link)
		if link.Code != codeB {
			t.Errorf("code = %q, want %q (the colliding draw must be skipped)", link.Code, codeB)
		}
		if link.ExpiresAt != "2025-03-01T13:00:00Z" {
			t.Errorf("expires_at = %q, want 2025-03-01T13:00:00Z", link.ExpiresAt)
		}
		if want := srv.URL + "/api/links/" + codeB + "/stats"; header.Get("Location") != want {
			t.Errorf("Location = %q, want %q", header.Get("Location"), want)
		}
	})

	t.Run("statistics of a link nobody followed", func(t *testing.T) {
		status, stats, _ := api.stats(codeA)
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200", status)
		}
		if stats.Hits != 0 || stats.LastAccessAt != "" || stats.Expired {
			t.Errorf("stats = %+v, want no hits, no last access, not expired", stats)
		}
	})

	t.Run("following links redirects and counts hits", func(t *testing.T) {
		clock.Advance(10 * time.Minute)
		for range 2 {
			status, header, _ := api.do(http.MethodGet, "/"+codeA, "", "")
			if status != http.StatusFound {
				t.Fatalf("GET /%s = %d, want 302", codeA, status)
			}
			if got := header.Get("Location"); got != targetA {
				t.Errorf("Location = %q, want %q", got, targetA)
			}
		}
		if status, _, _ := api.do(http.MethodGet, "/"+codeB, "", ""); status != http.StatusFound {
			t.Fatalf("GET /%s = %d, want 302", codeB, status)
		}

		_, statsA, _ := api.stats(codeA)
		if statsA.Hits != 2 || statsA.LastAccessAt != "2025-03-01T12:10:00Z" {
			t.Errorf("stats of A = %+v, want 2 hits, last access at 12:10", statsA)
		}
		_, statsB, _ := api.stats(codeB)
		if statsB.Hits != 1 {
			t.Errorf("stats of B = %+v, want 1 hit", statsB)
		}
	})

	t.Run("expired links answer 410 but keep their statistics", func(t *testing.T) {
		clock.Advance(time.Hour) // 13:10, past B's expiry at 13:00

		status, _, data := api.do(http.MethodGet, "/"+codeB, "", "")
		if status != http.StatusGone {
			t.Fatalf("GET /%s = %d, want 410", codeB, status)
		}
		var apiErr apiError
		mustUnmarshal(t, data, &apiErr)
		if apiErr.Error.Code != "link_expired" {
			t.Errorf("error code = %q, want link_expired", apiErr.Error.Code)
		}

		_, stats, _ := api.stats(codeB)
		if !stats.Expired || stats.Hits != 1 || stats.ExpiresAt != "2025-03-01T13:00:00Z" {
			t.Errorf("stats of B = %+v, want expired with its single hit kept", stats)
		}

		if status, _, _ := api.do(http.MethodGet, "/"+codeA, "", ""); status != http.StatusFound {
			t.Errorf("the permanent link stopped working: status %d", status)
		}
	})

	t.Run("error responses", func(t *testing.T) {
		tests := []struct {
			name        string
			method      string
			path        string
			contentType string
			body        string
			wantStatus  int
			wantCode    string
		}{
			{"unknown code", "GET", "/zzzzzzz", "", "", 404, "not_found"},
			{"malformed code", "GET", "/favicon.ico", "", "", 404, "not_found"},
			{"stats of unknown code", "GET", "/api/links/zzzzzzz/stats", "", "", 404, "not_found"},
			{"missing content type", "POST", "/api/links", "", `{"url":"https://example.com"}`, 415, "unsupported_media_type"},
			{"malformed JSON", "POST", "/api/links", "application/json", `{"url":`, 400, "invalid_json"},
			{"unknown field", "POST", "/api/links", "application/json", `{"url":"https://example.com","x":1}`, 400, "invalid_json"},
			{"javascript URL", "POST", "/api/links", "application/json", `{"url":"javascript:alert(1)"}`, 422, "invalid_url"},
			{"missing URL", "POST", "/api/links", "application/json", `{}`, 422, "invalid_url"},
			{"URL with credentials", "POST", "/api/links", "application/json", `{"url":"https://user:pw@example.com"}`, 422, "invalid_url"},
			{"zero TTL", "POST", "/api/links", "application/json", `{"url":"https://example.com","ttl_seconds":0}`, 422, "invalid_ttl"},
			{"TTL above one year", "POST", "/api/links", "application/json", `{"url":"https://example.com","ttl_seconds":31536001}`, 422, "invalid_ttl"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				status, header, data := api.do(tt.method, tt.path, tt.contentType, tt.body)
				if status != tt.wantStatus {
					t.Fatalf("status = %d, want %d\nbody: %s", status, tt.wantStatus, data)
				}
				if ct := header.Get("Content-Type"); ct != "application/json" {
					t.Errorf("Content-Type = %q, want application/json", ct)
				}
				var apiErr apiError
				mustUnmarshal(t, data, &apiErr)
				if apiErr.Error.Code != tt.wantCode || apiErr.Error.Message == "" {
					t.Errorf("error = %+v, want code %q with a message", apiErr.Error, tt.wantCode)
				}
			})
		}
	})

	t.Run("wrong methods are refused", func(t *testing.T) {
		for _, tt := range []struct{ method, path string }{
			{"GET", "/api/links"},
			{"DELETE", "/" + codeA},
			{"POST", "/docs"},
		} {
			if status, _, _ := api.do(tt.method, tt.path, "", ""); status != http.StatusMethodNotAllowed {
				t.Errorf("%s %s = %d, want 405", tt.method, tt.path, status)
			}
		}
	})
}
