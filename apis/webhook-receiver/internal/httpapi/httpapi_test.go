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
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brunofbenicio/go-lab/apis/webhook-receiver/internal/idempotency"
	"github.com/brunofbenicio/go-lab/apis/webhook-receiver/internal/replay"
	"github.com/brunofbenicio/go-lab/apis/webhook-receiver/internal/signature"
)

var (
	t0 = time.Date(2025, time.March, 1, 12, 0, 0, 0, time.UTC)
	// testSecret is an obviously fake key used only by these tests.
	testSecret = []byte("whsec_test_only_not_a_real_secret")
)

const (
	tolerance = 5 * time.Minute
	dedupeTTL = time.Hour
	goodBody  = `{"type":"order.created","data":{"id":42}}`
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

// recorder is a Processor that remembers the events it receives.
type recorder struct {
	mu     sync.Mutex
	events []Event
	err    error // returned by Process while non-nil
}

func (r *recorder) Process(_ context.Context, e Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
	return r.err
}

func (r *recorder) setErr(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}

// env is a fully wired handler on top of the real collaborators.
type env struct {
	mux   *http.ServeMux
	clock *clock
	proc  *recorder
	logs  *bytes.Buffer
}

func newEnv(t *testing.T, opts ...Option) *env {
	t.Helper()
	clk := &clock{t: t0}
	dedupe, err := idempotency.New(dedupeTTL, idempotency.WithClock(clk.Now))
	if err != nil {
		t.Fatal(err)
	}
	checker, err := replay.NewChecker(tolerance, replay.WithClock(clk.Now))
	if err != nil {
		t.Fatal(err)
	}
	proc := &recorder{}
	logs := &bytes.Buffer{}

	opts = append([]Option{WithClock(clk.Now), WithLogger(slog.New(slog.NewTextHandler(logs, nil)))}, opts...)
	h, err := New(Config{Secret: testSecret, Dedupe: dedupe, Timestamps: checker, Processor: proc}, opts...)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	mux := http.NewServeMux()
	h.Register(mux)
	return &env{mux: mux, clock: clk, proc: proc, logs: logs}
}

// stamp returns the current time of the test clock as a timestamp header.
func (e *env) stamp() string { return strconv.FormatInt(e.clock.Now().Unix(), 10) }

// signedRequest builds a correctly signed delivery.
func signedRequest(id, timestamp, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/webhooks", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderID, id)
	req.Header.Set(HeaderTimestamp, timestamp)
	req.Header.Set(HeaderSignature, signature.Sign(testSecret, signature.Message(id, timestamp, []byte(body))))
	return req
}

func (e *env) serve(req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, req)
	return rec
}

// deliver sends a correctly signed, fresh delivery.
func (e *env) deliver(id, body string) *httptest.ResponseRecorder {
	return e.serve(signedRequest(id, e.stamp(), body))
}

func assertError(t *testing.T, rec *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if rec.Code != wantStatus {
		t.Fatalf("status = %d, want %d\nbody: %s", rec.Code, wantStatus, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body errorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not valid JSON: %v\n%s", err, rec.Body.String())
	}
	if body.Error.Code != wantCode {
		t.Errorf("error code = %q, want %q (message %q)", body.Error.Code, wantCode, body.Error.Message)
	}
	if body.Error.Message == "" {
		t.Error("error message is empty")
	}
}

func TestRoutesAreRegistered(t *testing.T) {
	e := newEnv(t)
	for _, pattern := range Routes() {
		method, path, _ := strings.Cut(pattern, " ")
		if _, got := e.mux.Handler(httptest.NewRequest(method, path, nil)); got != pattern {
			t.Errorf("%s %s matched pattern %q, want %q", method, path, got, pattern)
		}
	}
}

func TestNewValidatesConfig(t *testing.T) {
	dedupe, err := idempotency.New(time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	checker, err := replay.NewChecker(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	proc := &recorder{}

	tests := []struct {
		name string
		cfg  Config
	}{
		{"no secret", Config{Dedupe: dedupe, Timestamps: checker, Processor: proc}},
		{"no dedupe", Config{Secret: testSecret, Timestamps: checker, Processor: proc}},
		{"no timestamp checker", Config{Secret: testSecret, Dedupe: dedupe, Processor: proc}},
		{"no processor", Config{Secret: testSecret, Dedupe: dedupe, Timestamps: checker}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.cfg); err == nil {
				t.Error("New() accepted an incomplete configuration")
			}
		})
	}
}

func TestAcceptedDelivery(t *testing.T) {
	e := newEnv(t)
	rec := e.deliver("evt_001", goodBody)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202\n%s", rec.Code, rec.Body.String())
	}
	var body acceptedResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != "accepted" || body.ID != "evt_001" {
		t.Errorf("body = %+v", body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}

	if e.proc.count() != 1 {
		t.Fatalf("processor called %d times, want 1", e.proc.count())
	}
	got := e.proc.events[0]
	if got.ID != "evt_001" || got.Type != "order.created" || string(got.Data) != `{"id":42}` || !got.ReceivedAt.Equal(t0) {
		t.Errorf("event = %+v", got)
	}
}

func TestAcceptedPayloads(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantType string
		wantData string // empty means Data must be nil
	}{
		{"object data", `{"type":"order.created","data":{"id":42,"items":[1,2]}}`, "order.created", `{"id":42,"items":[1,2]}`},
		{"no data", `{"type":"ping"}`, "ping", ""},
		{"null data", `{"type":"ping","data":null}`, "ping", ""},
		{"scalar data", `{"type":"ping","data":"hello"}`, "ping", `"hello"`},
		{"array data", `{"type":"ping","data":[1,2,3]}`, "ping", `[1,2,3]`},
		{"unknown top-level fields are tolerated", `{"type":"ping","version":2,"data":{}}`, "ping", `{}`},
		{"surrounding whitespace", " \n{\"type\":\"ping\"}\n ", "ping", ""},
		{"type with every allowed character", `{"type":"a0.b_c-d"}`, "a0.b_c-d", ""},
		{"type of exactly 100 characters", `{"type":"` + strings.Repeat("a", 100) + `"}`, strings.Repeat("a", 100), ""},
		{"non-ASCII data", `{"type":"user.created","data":{"name":"Zoë 😀"}}`, "user.created", `{"name":"Zoë 😀"}`},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			if rec := e.deliver(fmt.Sprintf("evt_%d", i), tt.body); rec.Code != http.StatusAccepted {
				t.Fatalf("status = %d, want 202\n%s", rec.Code, rec.Body.String())
			}
			got := e.proc.events[0]
			if got.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tt.wantType)
			}
			if tt.wantData == "" && got.Data != nil {
				t.Errorf("Data = %s, want nil", got.Data)
			}
			if tt.wantData != "" && string(got.Data) != tt.wantData {
				t.Errorf("Data = %s, want %s", got.Data, tt.wantData)
			}
		})
	}
}

func TestContentTypeIsNotRequired(t *testing.T) {
	e := newEnv(t)
	req := signedRequest("evt_1", e.stamp(), goodBody)
	req.Header.Del("Content-Type")
	if rec := e.serve(req); rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202", rec.Code)
	}
}

func TestInvalidPayloads(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"empty body", ``},
		{"blank body", " \n\t "},
		{"not JSON", `order.created`},
		{"truncated JSON", `{"type":"order.cre`},
		{"trailing garbage", `{"type":"ping"} garbage`},
		{"two JSON values", `{"type":"ping"}{"type":"pong"}`},
		{"JSON array", `[{"type":"ping"}]`},
		{"JSON string", `"ping"`},
		{"JSON null", `null`},
		{"missing type", `{"data":{}}`},
		{"empty type", `{"type":""}`},
		{"type is a number", `{"type":42}`},
		{"type is null", `{"type":null}`},
		{"type with uppercase letters", `{"type":"Order.Created"}`},
		{"type with a space", `{"type":"order created"}`},
		{"type starting with a dot", `{"type":".hidden"}`},
		{"type with a slash", `{"type":"order/created"}`},
		{"type of 101 characters", `{"type":"` + strings.Repeat("a", 101) + `"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			assertError(t, e.deliver("evt_1", tt.body), http.StatusBadRequest, "invalid_payload")
			if e.proc.count() != 0 {
				t.Error("the processor ran for an invalid payload")
			}
			// An invalid payload must not burn the delivery ID.
			if rec := e.deliver("evt_1", goodBody); rec.Code != http.StatusAccepted {
				t.Errorf("a corrected resend was refused with %d", rec.Code)
			}
		})
	}
}

func TestMissingSignature(t *testing.T) {
	e := newEnv(t)
	req := signedRequest("evt_1", e.stamp(), goodBody)
	req.Header.Del(HeaderSignature)

	assertError(t, e.serve(req), http.StatusUnauthorized, "missing_signature")
}

func TestRejectedSignatures(t *testing.T) {
	e := newEnv(t)
	ts := e.stamp()
	valid := signature.Sign(testSecret, signature.Message("evt_1", ts, []byte(goodBody)))
	hexPart := strings.TrimPrefix(valid, signature.Prefix)
	otherSecret := signature.Sign([]byte("another-fake-secret"), signature.Message("evt_1", ts, []byte(goodBody)))

	tests := []struct {
		name   string
		mutate func(*http.Request)
	}{
		{"signed with another secret", func(r *http.Request) { r.Header.Set(HeaderSignature, otherSecret) }},
		{"body replaced after signing", func(r *http.Request) { r.Body = io.NopCloser(strings.NewReader(`{"type":"order.deleted"}`)) }},
		{"delivery ID replaced after signing", func(r *http.Request) { r.Header.Set(HeaderID, "evt_2") }},
		{"timestamp replaced after signing", func(r *http.Request) {
			r.Header.Set(HeaderTimestamp, strconv.FormatInt(t0.Unix()+1, 10))
		}},
		{"digest without the prefix", func(r *http.Request) { r.Header.Set(HeaderSignature, hexPart) }},
		{"other algorithm", func(r *http.Request) { r.Header.Set(HeaderSignature, "sha1="+hexPart) }},
		{"truncated digest", func(r *http.Request) { r.Header.Set(HeaderSignature, valid[:len(valid)-2]) }},
		{"not hexadecimal", func(r *http.Request) { r.Header.Set(HeaderSignature, signature.Prefix+strings.Repeat("g", 64)) }},
		{"all zero digest", func(r *http.Request) { r.Header.Set(HeaderSignature, signature.Prefix+strings.Repeat("0", 64)) }},
		{"signature of an empty key", func(r *http.Request) {
			r.Header.Set(HeaderSignature, signature.Sign(nil, signature.Message("evt_1", ts, []byte(goodBody))))
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := signedRequest("evt_1", ts, goodBody)
			tt.mutate(req)
			rec := e.serve(req)

			assertError(t, rec, http.StatusUnauthorized, "invalid_signature")
			if strings.Contains(strings.ToLower(rec.Body.String()), "mismatch") || strings.Contains(rec.Body.String(), "malformed") {
				t.Errorf("the response tells the caller why verification failed: %s", rec.Body.String())
			}
		})
	}

	if e.proc.count() != 0 {
		t.Errorf("the processor ran %d times for unauthenticated requests", e.proc.count())
	}
	// None of the failed attempts may have consumed the delivery ID.
	if rec := e.serve(signedRequest("evt_1", ts, goodBody)); rec.Code != http.StatusAccepted {
		t.Errorf("a genuine delivery was refused with %d after forged attempts", rec.Code)
	}
}

func TestMissingHeaders(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{"delivery ID", HeaderID},
		{"timestamp", HeaderTimestamp},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			req := signedRequest("evt_1", e.stamp(), goodBody)
			req.Header.Del(tt.header)
			rec := e.serve(req)

			assertError(t, rec, http.StatusBadRequest, "missing_header")
			if !strings.Contains(rec.Body.String(), tt.header) {
				t.Errorf("the message does not name the missing header %s: %s", tt.header, rec.Body.String())
			}
		})
	}
}

func TestDeliveryIDValidation(t *testing.T) {
	tests := []struct {
		id string
		ok bool
	}{
		{"evt_001", true},
		{"01HZX3K9Y2W8", true},
		{"evt:2025.03-01_a", true},
		{"a", true},
		{strings.Repeat("a", 128), true},
		{strings.Repeat("a", 129), false},
		{"has space", false},
		{"-leading-dash", false},
		{".leading-dot", false},
		{"slash/inside", false},
		{"quote\"inside", false},
		{"non-ascii-é", false},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			e := newEnv(t)
			rec := e.deliver(tt.id, goodBody)
			if tt.ok {
				if rec.Code != http.StatusAccepted {
					t.Errorf("status = %d, want 202\n%s", rec.Code, rec.Body.String())
				}
				return
			}
			assertError(t, rec, http.StatusBadRequest, "invalid_webhook_id")
		})
	}
}

func TestTimestampTolerance(t *testing.T) {
	tests := []struct {
		name      string
		timestamp func(now time.Time) string
		wantCode  int
		wantError string
	}{
		{"now", func(now time.Time) string { return unixString(now) }, 202, ""},
		{"exactly at the past limit", func(now time.Time) string { return unixString(now.Add(-tolerance)) }, 202, ""},
		{"exactly at the future limit", func(now time.Time) string { return unixString(now.Add(tolerance)) }, 202, ""},
		{"one second too old", func(now time.Time) string { return unixString(now.Add(-tolerance - time.Second)) }, 401, "timestamp_out_of_tolerance"},
		{"one second too far ahead", func(now time.Time) string { return unixString(now.Add(tolerance + time.Second)) }, 401, "timestamp_out_of_tolerance"},
		{"a day old", func(now time.Time) string { return unixString(now.Add(-24 * time.Hour)) }, 401, "timestamp_out_of_tolerance"},
		{"not a number", func(time.Time) string { return "yesterday" }, 400, "invalid_timestamp"},
		{"milliseconds instead of seconds", func(now time.Time) string { return unixString(now) + "000" }, 400, "invalid_timestamp"},
		{"fractional seconds", func(now time.Time) string { return unixString(now) + ".5" }, 400, "invalid_timestamp"},
		{"negative", func(time.Time) string { return "-1" }, 400, "invalid_timestamp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			// The signature is valid in every case: only the timestamp varies.
			rec := e.serve(signedRequest("evt_1", tt.timestamp(e.clock.Now()), goodBody))
			if tt.wantError == "" {
				if rec.Code != tt.wantCode {
					t.Errorf("status = %d, want %d\n%s", rec.Code, tt.wantCode, rec.Body.String())
				}
				return
			}
			assertError(t, rec, tt.wantCode, tt.wantError)
			if e.proc.count() != 0 {
				t.Error("the processor ran for a rejected timestamp")
			}
		})
	}
}

func unixString(t time.Time) string { return strconv.FormatInt(t.Unix(), 10) }

func TestCapturedRequestCannotBeReplayedLater(t *testing.T) {
	e := newEnv(t)
	captured := func() *http.Request { return signedRequest("evt_1", e.stamp(), goodBody) }

	// The original delivery is accepted...
	if rec := e.serve(captured()); rec.Code != http.StatusAccepted {
		t.Fatalf("original delivery: status = %d", rec.Code)
	}
	// ...but the same signed request, replayed after the tolerance window and
	// well within the duplicate-detection TTL, is refused on its timestamp.
	staleTimestamp := e.stamp()
	e.clock.Advance(tolerance + time.Minute)
	rec := e.serve(signedRequest("evt_1", staleTimestamp, goodBody))
	assertError(t, rec, http.StatusUnauthorized, "timestamp_out_of_tolerance")

	// Forging a fresh timestamp breaks the signature.
	forged := signedRequest("evt_1", staleTimestamp, goodBody)
	forged.Header.Set(HeaderTimestamp, e.stamp())
	assertError(t, e.serve(forged), http.StatusUnauthorized, "invalid_signature")

	if e.proc.count() != 1 {
		t.Errorf("processor called %d times, want 1", e.proc.count())
	}
}

func TestDuplicateDelivery(t *testing.T) {
	e := newEnv(t)

	if rec := e.deliver("evt_1", goodBody); rec.Code != http.StatusAccepted {
		t.Fatalf("first delivery: status = %d", rec.Code)
	}
	assertError(t, e.deliver("evt_1", goodBody), http.StatusConflict, "duplicate_delivery")
	// The ID decides, not the content.
	assertError(t, e.deliver("evt_1", `{"type":"order.updated"}`), http.StatusConflict, "duplicate_delivery")
	// A different ID with identical content is a different delivery.
	if rec := e.deliver("evt_2", goodBody); rec.Code != http.StatusAccepted {
		t.Errorf("distinct delivery: status = %d", rec.Code)
	}

	if e.proc.count() != 2 {
		t.Errorf("processor called %d times, want 2", e.proc.count())
	}
}

func TestDeliveryIDExpires(t *testing.T) {
	e := newEnv(t)
	if rec := e.deliver("evt_1", goodBody); rec.Code != http.StatusAccepted {
		t.Fatal("first delivery refused")
	}

	e.clock.Advance(dedupeTTL - time.Second)
	assertError(t, e.deliver("evt_1", goodBody), http.StatusConflict, "duplicate_delivery")

	e.clock.Advance(time.Second)
	if rec := e.deliver("evt_1", goodBody); rec.Code != http.StatusAccepted {
		t.Errorf("after the TTL the ID should be accepted again, got %d", rec.Code)
	}
}

func TestFailedProcessingAllowsRetry(t *testing.T) {
	e := newEnv(t)
	e.proc.setErr(errors.New("database db-secret-host is down"))

	rec := e.deliver("evt_1", goodBody)
	assertError(t, rec, http.StatusInternalServerError, "processing_failed")
	if strings.Contains(rec.Body.String(), "db-secret-host") {
		t.Errorf("internal details leaked to the sender: %s", rec.Body.String())
	}
	if !strings.Contains(e.logs.String(), "db-secret-host") || !strings.Contains(e.logs.String(), "evt_1") {
		t.Errorf("the failure was not logged with its cause and delivery ID: %s", e.logs.String())
	}

	// The sender retries the same delivery once the receiver has recovered.
	e.proc.setErr(nil)
	if rec := e.deliver("evt_1", goodBody); rec.Code != http.StatusAccepted {
		t.Fatalf("retry: status = %d, want 202\n%s", rec.Code, rec.Body.String())
	}
	assertError(t, e.deliver("evt_1", goodBody), http.StatusConflict, "duplicate_delivery")

	if e.proc.count() != 2 {
		t.Errorf("processor called %d times, want 2 (the failed attempt and the retry)", e.proc.count())
	}
}

func TestConcurrentDuplicateDeliveries(t *testing.T) {
	const workers = 24
	e := newEnv(t)
	ts := e.stamp()

	codes := make(chan int, workers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := signedRequest("evt_race", ts, goodBody)
			<-start
			codes <- e.serve(req).Code
		}()
	}
	close(start)
	wg.Wait()
	close(codes)

	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[http.StatusAccepted] != 1 || counts[http.StatusConflict] != workers-1 {
		t.Errorf("status counts = %v, want one 202 and %d 409s", counts, workers-1)
	}
	if e.proc.count() != 1 {
		t.Errorf("processor called %d times, want 1", e.proc.count())
	}
}

func TestBodyLimit(t *testing.T) {
	const limit = 200
	// padded builds a valid payload of exactly n bytes.
	padded := func(n int) string {
		const frame = `{"type":"ping","data":""}`
		return `{"type":"ping","data":"` + strings.Repeat("x", n-len(frame)) + `"}`
	}

	t.Run("a body of exactly the limit is accepted", func(t *testing.T) {
		e := newEnv(t, WithMaxBodyBytes(limit))
		body := padded(limit)
		if len(body) != limit {
			t.Fatalf("test body has %d bytes, want %d", len(body), limit)
		}
		if rec := e.deliver("evt_1", body); rec.Code != http.StatusAccepted {
			t.Errorf("status = %d, want 202\n%s", rec.Code, rec.Body.String())
		}
	})

	t.Run("one byte over the limit is refused even when correctly signed", func(t *testing.T) {
		e := newEnv(t, WithMaxBodyBytes(limit))
		assertError(t, e.deliver("evt_1", padded(limit+1)), http.StatusRequestEntityTooLarge, "payload_too_large")
		if e.proc.count() != 0 {
			t.Error("the processor ran for an oversized body")
		}
	})

	t.Run("a chunked body of unknown length is cut off too", func(t *testing.T) {
		e := newEnv(t, WithMaxBodyBytes(limit))
		req := signedRequest("evt_1", e.stamp(), padded(limit*10))
		req.ContentLength = -1
		assertError(t, e.serve(req), http.StatusRequestEntityTooLarge, "payload_too_large")
	})

	t.Run("a declared length over the limit is refused without reading the body", func(t *testing.T) {
		e := newEnv(t, WithMaxBodyBytes(limit))
		req := signedRequest("evt_1", e.stamp(), goodBody)
		req.Body = io.NopCloser(failingReader{t})
		req.ContentLength = limit + 1
		assertError(t, e.serve(req), http.StatusRequestEntityTooLarge, "payload_too_large")
	})

	t.Run("the limit applies before authentication", func(t *testing.T) {
		e := newEnv(t, WithMaxBodyBytes(limit))
		req := httptest.NewRequest(http.MethodPost, "/webhooks", strings.NewReader(strings.Repeat("x", limit+1)))
		assertError(t, e.serve(req), http.StatusRequestEntityTooLarge, "payload_too_large")
	})

	t.Run("the default limit is 64 KiB", func(t *testing.T) {
		e := newEnv(t, WithMaxBodyBytes(0), WithMaxBodyBytes(-5))
		assertError(t, e.deliver("evt_1", padded(DefaultMaxBodyBytes+1)), http.StatusRequestEntityTooLarge, "payload_too_large")
		if rec := e.deliver("evt_2", padded(DefaultMaxBodyBytes)); rec.Code != http.StatusAccepted {
			t.Errorf("a body of exactly 64 KiB was refused with %d", rec.Code)
		}
	})
}

// failingReader fails the test if anything reads from it.
type failingReader struct{ t *testing.T }

func (f failingReader) Read([]byte) (int, error) {
	f.t.Helper()
	f.t.Error("the request body was read although its declared length is over the limit")
	return 0, io.EOF
}

// brokenReader fails every read, like a client that hung up mid-request.
type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, errors.New("connection reset") }

func TestUnreadableBody(t *testing.T) {
	e := newEnv(t)
	req := signedRequest("evt_1", e.stamp(), goodBody)
	req.Body = io.NopCloser(brokenReader{})
	req.ContentLength = -1

	assertError(t, e.serve(req), http.StatusBadRequest, "invalid_payload")
}

func TestMethodNotAllowed(t *testing.T) {
	e := newEnv(t)
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			rec := e.serve(httptest.NewRequest(method, "/webhooks", nil))
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want 405", rec.Code)
			}
			if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "POST") {
				t.Errorf("Allow = %q, want it to contain POST", allow)
			}
		})
	}
}

func TestOptionsIgnoreNilValues(t *testing.T) {
	e := newEnv(t, WithClock(nil), WithLogger(nil))
	if rec := e.deliver("evt_1", goodBody); rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202", rec.Code)
	}
}

func TestProcessorFunc(t *testing.T) {
	var got Event
	f := ProcessorFunc(func(_ context.Context, e Event) error {
		got = e
		return nil
	})
	if err := f.Process(context.Background(), Event{ID: "evt_1"}); err != nil || got.ID != "evt_1" {
		t.Errorf("Process() = %v, event %+v", err, got)
	}
}

func TestSecretIsCopied(t *testing.T) {
	secret := append([]byte(nil), testSecret...)
	clk := &clock{t: t0}
	dedupe, _ := idempotency.New(time.Hour, idempotency.WithClock(clk.Now))
	checker, _ := replay.NewChecker(time.Hour, replay.WithClock(clk.Now))
	h, err := New(Config{Secret: secret, Dedupe: dedupe, Timestamps: checker, Processor: &recorder{}})
	if err != nil {
		t.Fatal(err)
	}
	for i := range secret {
		secret[i] = 0 // the caller wipes its copy
	}
	mux := http.NewServeMux()
	h.Register(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, signedRequest("evt_1", unixString(t0), goodBody))
	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d; the handler must keep its own copy of the secret", rec.Code)
	}
}
