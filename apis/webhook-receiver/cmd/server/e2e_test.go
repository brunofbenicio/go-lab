package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brunofbenicio/go-lab/apis/webhook-receiver/internal/httpapi"
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

// recordingProcessor remembers events and can be told to fail.
type recordingProcessor struct {
	mu     sync.Mutex
	events []httpapi.Event
	err    error
}

func (p *recordingProcessor) Process(_ context.Context, e httpapi.Event) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, e)
	return p.err
}

func (p *recordingProcessor) setErr(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.err = err
}

func (p *recordingProcessor) snapshot() []httpapi.Event {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]httpapi.Event(nil), p.events...)
}

// delivery is a request as a webhook sender would build it. The signature is
// computed here with crypto/hmac directly, independently of the receiver's own
// signature package, so the test checks the documented wire format.
type delivery struct {
	id        string
	timestamp string
	body      string
	signature string // overrides the computed signature when non-empty
	unsigned  bool   // omits the signature header altogether
}

func sign(secret, id, timestamp, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(id + "." + timestamp + "." + body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

type response struct {
	status int
	body   []byte
}

func (r response) errorCode(t *testing.T) string {
	t.Helper()
	var e struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(r.body, &e); err != nil {
		t.Fatalf("response is not valid JSON: %v\nbody: %s", err, r.body)
	}
	if e.Error.Message == "" {
		t.Errorf("error response without a message: %s", r.body)
	}
	return e.Error.Code
}

// TestEndToEnd walks through the receiver over real HTTP: accepting a signed
// delivery, refusing forgeries, replays and duplicates, recovering from a
// processing failure and enforcing the body limit.
func TestEndToEnd(t *testing.T) {
	clock := &fakeClock{t: time.Date(2025, time.March, 1, 12, 0, 0, 0, time.UTC)}
	proc := &recordingProcessor{}
	cfg, err := configFromEnv(envFrom(map[string]string{
		"WEBHOOK_SECRET": testSecret, "MAX_BODY_BYTES": "512",
		"TIMESTAMP_TOLERANCE": "5m", "IDEMPOTENCY_TTL": "24h",
	}))
	if err != nil {
		t.Fatal(err)
	}
	handler, err := newHandler(cfg, discardLogger(), deps{now: clock.Now, processor: proc})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	tr := &http.Transport{DisableKeepAlives: true}
	t.Cleanup(tr.CloseIdleConnections)
	client := &http.Client{Transport: tr, Timeout: 10 * time.Second}

	now := func() string { return strconv.FormatInt(clock.Now().Unix(), 10) }
	send := func(t *testing.T, d delivery) response {
		t.Helper()
		req, err := http.NewRequest(http.MethodPost, srv.URL+"/webhooks", strings.NewReader(d.body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		if d.id != "" {
			req.Header.Set("X-Webhook-Id", d.id)
		}
		if d.timestamp != "" {
			req.Header.Set("X-Webhook-Timestamp", d.timestamp)
		}
		switch {
		case d.unsigned:
		case d.signature != "":
			req.Header.Set("X-Webhook-Signature", d.signature)
		case d.id != "" && d.timestamp != "":
			req.Header.Set("X-Webhook-Signature", sign(testSecret, d.id, d.timestamp, d.body))
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response{status: resp.StatusCode, body: data}
	}

	const body = `{"type":"order.created","data":{"id":42,"total":99.9}}`

	t.Run("documentation is served", func(t *testing.T) {
		for path, marker := range map[string]string{"/openapi.yaml": "openapi: 3.0.3", "/docs": "swagger-ui-dist@5.17.14"} {
			resp, err := client.Get(srv.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			data, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK || !strings.Contains(string(data), marker) {
				t.Errorf("GET %s = %d, missing %q", path, resp.StatusCode, marker)
			}
		}
	})

	t.Run("a signed delivery is accepted", func(t *testing.T) {
		resp := send(t, delivery{id: "evt_001", timestamp: now(), body: body})
		if resp.status != http.StatusAccepted {
			t.Fatalf("status = %d, want 202\n%s", resp.status, resp.body)
		}
		if got := strings.TrimSpace(string(resp.body)); got != `{"status":"accepted","id":"evt_001"}` {
			t.Errorf("body = %s", got)
		}

		events := proc.snapshot()
		if len(events) != 1 {
			t.Fatalf("processor saw %d events, want 1", len(events))
		}
		if e := events[0]; e.ID != "evt_001" || e.Type != "order.created" ||
			string(e.Data) != `{"id":42,"total":99.9}` || !e.ReceivedAt.Equal(clock.Now()) {
			t.Errorf("event = %+v", e)
		}
	})

	t.Run("a retry of the same delivery is a duplicate, even with a fresh signature", func(t *testing.T) {
		clock.Advance(time.Minute)
		resp := send(t, delivery{id: "evt_001", timestamp: now(), body: body})
		if resp.status != http.StatusConflict || resp.errorCode(t) != "duplicate_delivery" {
			t.Errorf("status = %d, body = %s", resp.status, resp.body)
		}
		if n := len(proc.snapshot()); n != 1 {
			t.Errorf("processor saw %d events, want it to stay at 1", n)
		}
	})

	t.Run("forged deliveries are refused and do not burn the ID", func(t *testing.T) {
		forgeries := []struct {
			name     string
			d        delivery
			wantCode string
		}{
			{"no signature", delivery{id: "evt_002", timestamp: now(), body: body, unsigned: true}, "missing_signature"},
			{"signature of another secret", delivery{id: "evt_002", timestamp: now(), body: body,
				signature: sign("another-fake-secret-value", "evt_002", now(), body)}, "invalid_signature"},
			{"body altered after signing", delivery{id: "evt_002", timestamp: now(), body: `{"type":"order.deleted"}`,
				signature: sign(testSecret, "evt_002", now(), body)}, "invalid_signature"},
			{"ID altered after signing", delivery{id: "evt_002", timestamp: now(), body: body,
				signature: sign(testSecret, "evt_999", now(), body)}, "invalid_signature"},
			{"signature over the body alone", delivery{id: "evt_002", timestamp: now(), body: body,
				signature: bodyOnlySignature(testSecret, body)}, "invalid_signature"},
		}
		for _, f := range forgeries {
			resp := send(t, f.d)
			if resp.status != http.StatusUnauthorized || resp.errorCode(t) != f.wantCode {
				t.Errorf("%s: status = %d, body = %s; want 401 %s", f.name, resp.status, resp.body, f.wantCode)
			}
		}

		if n := len(proc.snapshot()); n != 1 {
			t.Errorf("processor saw %d events, want it to stay at 1", n)
		}
		resp := send(t, delivery{id: "evt_002", timestamp: now(), body: body})
		if resp.status != http.StatusAccepted {
			t.Errorf("the genuine delivery was refused after the forgeries: %d %s", resp.status, resp.body)
		}
	})

	t.Run("a captured request cannot be replayed after the tolerance window", func(t *testing.T) {
		captured := delivery{id: "evt_003", timestamp: now(), body: body}
		if resp := send(t, captured); resp.status != http.StatusAccepted {
			t.Fatalf("original: status = %d", resp.status)
		}

		// Past the tolerance but long before the duplicate memory expires, so
		// only the timestamp check stands between the attacker and a second run.
		clock.Advance(6 * time.Minute)
		resp := send(t, captured)
		if resp.status != http.StatusUnauthorized || resp.errorCode(t) != "timestamp_out_of_tolerance" {
			t.Errorf("replay: status = %d, body = %s", resp.status, resp.body)
		}

		// Refreshing the timestamp invalidates the signature.
		refreshed := captured
		refreshed.timestamp = now()
		refreshed.signature = sign(testSecret, captured.id, captured.timestamp, captured.body)
		resp = send(t, refreshed)
		if resp.status != http.StatusUnauthorized || resp.errorCode(t) != "invalid_signature" {
			t.Errorf("replay with a fresh timestamp: status = %d, body = %s", resp.status, resp.body)
		}
	})

	t.Run("timestamps from the future are refused beyond the tolerance", func(t *testing.T) {
		future := strconv.FormatInt(clock.Now().Add(6*time.Minute).Unix(), 10)
		resp := send(t, delivery{id: "evt_004", timestamp: future, body: body})
		if resp.status != http.StatusUnauthorized || resp.errorCode(t) != "timestamp_out_of_tolerance" {
			t.Errorf("status = %d, body = %s", resp.status, resp.body)
		}
	})

	t.Run("a failed delivery can be retried", func(t *testing.T) {
		before := len(proc.snapshot())
		proc.setErr(errors.New("downstream is unavailable"))
		resp := send(t, delivery{id: "evt_005", timestamp: now(), body: body})
		if resp.status != http.StatusInternalServerError || resp.errorCode(t) != "processing_failed" {
			t.Fatalf("failing delivery: status = %d, body = %s", resp.status, resp.body)
		}
		if strings.Contains(string(resp.body), "downstream") {
			t.Errorf("internal details leaked: %s", resp.body)
		}

		proc.setErr(nil)
		resp = send(t, delivery{id: "evt_005", timestamp: now(), body: body})
		if resp.status != http.StatusAccepted {
			t.Fatalf("retry: status = %d, body = %s", resp.status, resp.body)
		}
		if n := len(proc.snapshot()) - before; n != 2 {
			t.Errorf("processor saw %d attempts, want 2 (the failure and the retry)", n)
		}
		if resp := send(t, delivery{id: "evt_005", timestamp: now(), body: body}); resp.status != http.StatusConflict {
			t.Errorf("third attempt: status = %d, want 409", resp.status)
		}
	})

	t.Run("the body limit is enforced", func(t *testing.T) {
		big := `{"type":"blob","data":"` + strings.Repeat("x", 600) + `"}`
		resp := send(t, delivery{id: "evt_006", timestamp: now(), body: big})
		if resp.status != http.StatusRequestEntityTooLarge || resp.errorCode(t) != "payload_too_large" {
			t.Errorf("status = %d, body = %s", resp.status, resp.body)
		}
		if !strings.Contains(string(resp.body), "512") {
			t.Errorf("the message should state the configured limit: %s", resp.body)
		}
	})

	t.Run("malformed requests", func(t *testing.T) {
		tests := []struct {
			name       string
			d          delivery
			wantStatus int
			wantCode   string
		}{
			{"missing delivery ID", delivery{timestamp: now(), body: body, signature: "sha256=" + strings.Repeat("0", 64)}, 400, "missing_header"},
			{"missing timestamp", delivery{id: "evt_007", body: body, signature: "sha256=" + strings.Repeat("0", 64)}, 400, "missing_header"},
			{"timestamp in milliseconds", delivery{id: "evt_007", timestamp: now() + "000", body: body}, 400, "invalid_timestamp"},
			{"body is not JSON", delivery{id: "evt_007", timestamp: now(), body: "hello"}, 400, "invalid_payload"},
			{"event without a type", delivery{id: "evt_007", timestamp: now(), body: `{"data":{}}`}, 400, "invalid_payload"},
			{"delivery ID with a space", delivery{id: "evt 007", timestamp: now(), body: body}, 400, "invalid_webhook_id"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				resp := send(t, tt.d)
				if resp.status != tt.wantStatus || resp.errorCode(t) != tt.wantCode {
					t.Errorf("status = %d, body = %s; want %d %s", resp.status, resp.body, tt.wantStatus, tt.wantCode)
				}
			})
		}
	})

	t.Run("delivery IDs are forgotten after the TTL", func(t *testing.T) {
		clock.Advance(24 * time.Hour)
		resp := send(t, delivery{id: "evt_001", timestamp: now(), body: body})
		if resp.status != http.StatusAccepted {
			t.Errorf("status = %d, want 202 once the duplicate memory has expired\n%s", resp.status, resp.body)
		}
	})

	t.Run("only POST is allowed", func(t *testing.T) {
		resp, err := client.Get(srv.URL + "/webhooks")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("GET /webhooks = %d, want 405", resp.StatusCode)
		}
	})
}

// bodyOnlySignature signs just the body, as a naive receiver would. The
// receiver must refuse it because it does not cover the ID and the timestamp.
func bodyOnlySignature(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
