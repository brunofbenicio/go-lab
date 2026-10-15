package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brunofbenicio/go-lab/apis/webhook-receiver/internal/httpapi"
)

// testSecret is an obviously fake key used only by these tests.
const testSecret = "whsec_test_only_not_a_real_secret"

func discardLogger() *slog.Logger { return slog.New(slog.DiscardHandler) }

func envFrom(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestConfigFromEnv(t *testing.T) {
	defaults := config{
		Port: "8080", MaxBodyBytes: 65536, Tolerance: 5 * time.Minute, DedupeTTL: 24 * time.Hour,
	}
	with := func(mutate func(*config)) config {
		c := defaults
		mutate(&c)
		return c
	}

	tests := []struct {
		name    string
		env     map[string]string
		want    config // Secret is compared separately
		wantErr string // substring of the expected error, empty when it must succeed
	}{
		{name: "defaults", env: map[string]string{"WEBHOOK_SECRET": testSecret}, want: defaults},
		{
			name: "everything customized",
			env: map[string]string{
				"WEBHOOK_SECRET": testSecret, "PORT": "9090", "MAX_BODY_BYTES": "1024",
				"TIMESTAMP_TOLERANCE": "1m", "IDEMPOTENCY_TTL": "2h",
			},
			want: config{Port: "9090", MaxBodyBytes: 1024, Tolerance: time.Minute, DedupeTTL: 2 * time.Hour},
		},
		{
			name: "port zero asks the OS for a free port",
			env:  map[string]string{"WEBHOOK_SECRET": testSecret, "PORT": "0"},
			want: with(func(c *config) { c.Port = "0" }),
		},
		{
			name: "secret of exactly 16 bytes",
			env:  map[string]string{"WEBHOOK_SECRET": strings.Repeat("k", 16)},
			want: defaults,
		},
		{
			name: "ttl of exactly twice the tolerance",
			env:  map[string]string{"WEBHOOK_SECRET": testSecret, "TIMESTAMP_TOLERANCE": "5m", "IDEMPOTENCY_TTL": "10m"},
			want: with(func(c *config) { c.DedupeTTL = 10 * time.Minute }),
		},
		{
			name: "largest body limit",
			env:  map[string]string{"WEBHOOK_SECRET": testSecret, "MAX_BODY_BYTES": "10485760"},
			want: with(func(c *config) { c.MaxBodyBytes = 10 << 20 }),
		},

		{name: "missing secret", env: nil, wantErr: "WEBHOOK_SECRET is required"},
		{name: "secret one byte short", env: map[string]string{"WEBHOOK_SECRET": strings.Repeat("k", 15)}, wantErr: "too short"},
		{name: "port is not a number", env: map[string]string{"WEBHOOK_SECRET": testSecret, "PORT": "http"}, wantErr: "PORT"},
		{name: "port is too large", env: map[string]string{"WEBHOOK_SECRET": testSecret, "PORT": "65536"}, wantErr: "PORT"},
		{name: "body limit is not a number", env: map[string]string{"WEBHOOK_SECRET": testSecret, "MAX_BODY_BYTES": "big"}, wantErr: "MAX_BODY_BYTES"},
		{name: "body limit is zero", env: map[string]string{"WEBHOOK_SECRET": testSecret, "MAX_BODY_BYTES": "0"}, wantErr: "MAX_BODY_BYTES"},
		{name: "body limit is negative", env: map[string]string{"WEBHOOK_SECRET": testSecret, "MAX_BODY_BYTES": "-1"}, wantErr: "MAX_BODY_BYTES"},
		{name: "body limit above 10 MiB", env: map[string]string{"WEBHOOK_SECRET": testSecret, "MAX_BODY_BYTES": "10485761"}, wantErr: "MAX_BODY_BYTES"},
		{name: "tolerance is not a duration", env: map[string]string{"WEBHOOK_SECRET": testSecret, "TIMESTAMP_TOLERANCE": "300"}, wantErr: "TIMESTAMP_TOLERANCE"},
		{name: "tolerance is zero", env: map[string]string{"WEBHOOK_SECRET": testSecret, "TIMESTAMP_TOLERANCE": "0s"}, wantErr: "TIMESTAMP_TOLERANCE"},
		{name: "tolerance above one hour", env: map[string]string{"WEBHOOK_SECRET": testSecret, "TIMESTAMP_TOLERANCE": "61m"}, wantErr: "TIMESTAMP_TOLERANCE"},
		{name: "ttl is not a duration", env: map[string]string{"WEBHOOK_SECRET": testSecret, "IDEMPOTENCY_TTL": "forever"}, wantErr: "IDEMPOTENCY_TTL"},
		{name: "ttl is negative", env: map[string]string{"WEBHOOK_SECRET": testSecret, "IDEMPOTENCY_TTL": "-1h"}, wantErr: "IDEMPOTENCY_TTL"},
		{name: "ttl above 30 days", env: map[string]string{"WEBHOOK_SECRET": testSecret, "IDEMPOTENCY_TTL": "721h"}, wantErr: "IDEMPOTENCY_TTL"},
		{
			name:    "ttl shorter than twice the tolerance",
			env:     map[string]string{"WEBHOOK_SECRET": testSecret, "TIMESTAMP_TOLERANCE": "5m", "IDEMPOTENCY_TTL": "9m59s"},
			wantErr: "at least twice",
		},
		{
			name:    "ttl equal to the tolerance",
			env:     map[string]string{"WEBHOOK_SECRET": testSecret, "TIMESTAMP_TOLERANCE": "1h", "IDEMPOTENCY_TTL": "1h"},
			wantErr: "at least twice",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := configFromEnv(envFrom(tt.env))
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("configFromEnv() error = %v, want it to contain %q", err, tt.wantErr)
				}
				if secret := tt.env["WEBHOOK_SECRET"]; secret != "" && strings.Contains(err.Error(), secret) {
					t.Errorf("the error message leaks the secret: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("configFromEnv() failed: %v", err)
			}
			if got.Port != tt.want.Port || got.MaxBodyBytes != tt.want.MaxBodyBytes ||
				got.Tolerance != tt.want.Tolerance || got.DedupeTTL != tt.want.DedupeTTL {
				t.Errorf("configFromEnv() = %+v, want %+v", got, tt.want)
			}
			if string(got.Secret) != tt.env["WEBHOOK_SECRET"] {
				t.Error("the secret was not taken from WEBHOOK_SECRET")
			}
		})
	}
}

func TestConfigLogValueHidesTheSecret(t *testing.T) {
	cfg, err := configFromEnv(envFrom(map[string]string{"WEBHOOK_SECRET": testSecret}))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	slog.New(slog.NewTextHandler(&out, nil)).Info("starting", "config", cfg)

	if strings.Contains(out.String(), testSecret) {
		t.Errorf("the secret was logged: %s", out.String())
	}
	if !strings.Contains(out.String(), "secret_set=true") {
		t.Errorf("log does not say that a secret is set: %s", out.String())
	}
}

func TestNewHandlerServesDocs(t *testing.T) {
	cfg, err := configFromEnv(envFrom(map[string]string{"WEBHOOK_SECRET": testSecret}))
	if err != nil {
		t.Fatal(err)
	}
	handler, err := newHandler(cfg, discardLogger(), deps{})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/docs", "/openapi.yaml"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", path, rec.Code)
		}
	}
}

func TestLogProcessorDoesNotLogThePayload(t *testing.T) {
	var logs bytes.Buffer
	p := logProcessor(slog.New(slog.NewTextHandler(&logs, nil)))

	data := []byte(`{"email":"alice@example.com"}`)
	err := p.Process(context.Background(), httpapi.Event{ID: "evt_1", Type: "user.created", Data: data})
	if err != nil {
		t.Fatalf("Process() = %v", err)
	}
	out := logs.String()
	for _, want := range []string{"evt_1", "user.created", fmt.Sprintf("data_bytes=%d", len(data))} {
		if !strings.Contains(out, want) {
			t.Errorf("log does not mention %q: %s", want, out)
		}
	}
	if strings.Contains(out, "alice@example.com") {
		t.Errorf("the payload was logged: %s", out)
	}
}

// waitUntilRefused polls addr until new connections are refused, which means
// the server stopped listening.
func waitUntilRefused(t *testing.T, addr string) {
	t.Helper()
	for range 3000 {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err != nil {
			return
		}
		conn.Close()
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("server at %s kept accepting connections after shutdown began", addr)
}

func newTestClient(t *testing.T) *http.Client {
	t.Helper()
	tr := &http.Transport{DisableKeepAlives: true}
	t.Cleanup(tr.CloseIdleConnections)
	return &http.Client{Transport: tr, Timeout: 10 * time.Second}
}

func TestServeShutsDownGracefully(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		fmt.Fprint(w, "finished")
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	served := make(chan error, 1)
	go func() { served <- serve(ctx, &http.Server{Handler: mux}, ln, 5*time.Second) }()

	type result struct {
		body string
		err  error
	}
	response := make(chan result, 1)
	go func() {
		resp, err := newTestClient(t).Get("http://" + addr + "/slow")
		if err != nil {
			response <- result{err: err}
			return
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		response <- result{body: string(b), err: err}
	}()

	<-started // the request is now in flight
	cancel()  // ask the server to stop
	waitUntilRefused(t, addr)

	select {
	case err := <-served:
		t.Fatalf("serve returned (%v) while a request was still in flight", err)
	default:
	}

	close(release)
	if got := <-response; got.err != nil || got.body != "finished" {
		t.Errorf("in-flight request = %q, %v; want it to complete with %q", got.body, got.err, "finished")
	}
	if err := <-served; err != nil {
		t.Errorf("serve() = %v, want nil after a clean shutdown", err)
	}
}

func TestServeGivesUpAfterGracePeriod(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("GET /stuck", func(http.ResponseWriter, *http.Request) {
		close(started)
		<-release
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	served := make(chan error, 1)
	go func() { served <- serve(ctx, &http.Server{Handler: mux}, ln, 20*time.Millisecond) }()

	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		resp, err := newTestClient(t).Get("http://" + ln.Addr().String() + "/stuck")
		if err == nil {
			resp.Body.Close()
		}
	}()

	<-started
	cancel()
	err = <-served
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("serve() = %v, want it to wrap context.DeadlineExceeded", err)
	}

	close(release)
	<-clientDone
}

func TestServeReturnsListenerFailure(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln.Close() // Serve will fail on the first Accept

	err = serve(context.Background(), &http.Server{Handler: http.NewServeMux()}, ln, time.Second)
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		t.Errorf("serve() = %v, want a listener error", err)
	}
}

func TestRun(t *testing.T) {
	t.Run("missing secret", func(t *testing.T) {
		err := run(context.Background(), envFrom(nil), discardLogger(), deps{})
		if err == nil || !strings.Contains(err.Error(), "WEBHOOK_SECRET") {
			t.Errorf("run() error = %v, want a complaint about WEBHOOK_SECRET", err)
		}
	})

	t.Run("port already in use", func(t *testing.T) {
		busy, err := net.Listen("tcp", ":0")
		if err != nil {
			t.Fatal(err)
		}
		defer busy.Close()
		port := fmt.Sprint(busy.Addr().(*net.TCPAddr).Port)

		err = run(context.Background(), envFrom(map[string]string{"WEBHOOK_SECRET": testSecret, "PORT": port}), discardLogger(), deps{})
		if err == nil {
			t.Fatal("run() succeeded on a port that is already taken")
		}
	})

	t.Run("starts and stops on context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		var logs bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logs, nil))
		if err := run(ctx, envFrom(map[string]string{"WEBHOOK_SECRET": testSecret, "PORT": "0"}), logger, deps{}); err != nil {
			t.Errorf("run() = %v, want nil", err)
		}
		if strings.Contains(logs.String(), testSecret) {
			t.Errorf("the secret was logged at startup: %s", logs.String())
		}
	})
}
