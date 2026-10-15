package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newServer starts a loopback server with a handful of endpoints.
func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/missing", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not here", http.StatusNotFound)
	})
	mux.HandleFunc("/boom", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "broken", http.StatusInternalServerError)
	})
	mux.HandleFunc("/teapot", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ok", http.StatusFound)
	})
	mux.HandleFunc("/head-only", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done(): // the client gave up
		case <-time.After(2 * time.Second): // safety net so the test cannot hang
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func runCLI(args ...string) (code int, stdout, stderr string) {
	var out, errOut bytes.Buffer
	code = run(args, strings.NewReader(""), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestRunStatuses(t *testing.T) {
	srv := newServer(t)
	u := srv.URL
	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  string
	}{
		{"success", []string{u + "/ok"}, 0, "OK   200 " + u + "/ok\n"},
		{"not found", []string{u + "/missing"}, 1, "FAIL 404 " + u + "/missing\n"},
		{"server error", []string{u + "/boom"}, 1, "FAIL 500 " + u + "/boom\n"},
		{
			"mixed results keep the input order", []string{u + "/boom", u + "/ok", u + "/missing"}, 1,
			"FAIL 500 " + u + "/boom\nOK   200 " + u + "/ok\nFAIL 404 " + u + "/missing\n",
		},
		{
			"single worker", []string{"-c", "1", u + "/ok", u + "/ok"}, 0,
			"OK   200 " + u + "/ok\nOK   200 " + u + "/ok\n",
		},
		{"redirects are followed", []string{u + "/redirect"}, 0, "OK   200 " + u + "/redirect\n"},
		{"redirects can be reported", []string{"-no-redirect", u + "/redirect"}, 0, "OK   302 " + u + "/redirect\n"},
		{"expected error status passes", []string{"-expect", "404", u + "/missing"}, 0, "OK   404 " + u + "/missing\n"},
		{"unexpected status fails", []string{"-expect", "404", u + "/ok"}, 1, "FAIL 200 " + u + "/ok\n"},
		{"unusual status as expected", []string{"-expect", "418", u + "/teapot"}, 0, "OK   418 " + u + "/teapot\n"},
		{"method flag is honored", []string{"-method", "head", u + "/head-only"}, 0, "OK   200 " + u + "/head-only\n"},
		{"default method is GET", []string{u + "/head-only"}, 1, "FAIL 405 " + u + "/head-only\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runCLI(tt.args...)
			if code != tt.wantCode || stdout != tt.wantOut || stderr != "" {
				t.Errorf("run(%q) = (%d, %q, %q), want (%d, %q, \"\")", tt.args, code, stdout, stderr, tt.wantCode, tt.wantOut)
			}
		})
	}
}

func TestRunNetworkFailures(t *testing.T) {
	srv := newServer(t)
	closed := httptest.NewServer(http.NotFoundHandler())
	closedURL := closed.URL
	closed.Close() // nothing listens on this loopback port any more

	tests := []struct {
		name       string
		args       []string
		wantPrefix string
		wantHas    string
	}{
		{"timeout", []string{"-timeout", "50ms", srv.URL + "/slow"}, "FAIL ERR " + srv.URL + "/slow (", "timeout after 50ms"},
		{"connection refused", []string{closedURL}, "FAIL ERR " + closedURL + " (", "refused"},
		{"unsupported scheme", []string{"ftp://example.com/file"}, "FAIL ERR ftp://example.com/file (", "invalid URL"},
		{"missing scheme", []string{"example.com"}, "FAIL ERR example.com (", "invalid URL"},
		{"empty URL", []string{""}, "FAIL ERR  (", "invalid URL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, _ := runCLI(tt.args...)
			if code != 1 || !strings.HasPrefix(stdout, tt.wantPrefix) || !strings.Contains(stdout, tt.wantHas) {
				t.Errorf("run(%q) = (%d, %q), want code 1 and output starting %q containing %q",
					tt.args, code, stdout, tt.wantPrefix, tt.wantHas)
			}
		})
	}
}

func TestRunUsage(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantErrHas string
	}{
		{"no URLs", nil, 2, "at least one URL"},
		{"zero timeout", []string{"-timeout", "0s", "http://127.0.0.1:1"}, 2, "-timeout must be positive"},
		{"expect too small", []string{"-expect", "99", "http://127.0.0.1:1"}, 2, "-expect must be"},
		{"expect too large", []string{"-expect", "600", "http://127.0.0.1:1"}, 2, "-expect must be"},
		{"no workers", []string{"-c", "0", "http://127.0.0.1:1"}, 2, "-c must be"},
		{"unknown flag", []string{"-nope"}, 2, "not defined"},
		{"bad duration", []string{"-timeout", "soon", "x"}, 2, "invalid value"},
		{"help", []string{"-h"}, 0, "usage: httpcheck"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runCLI(tt.args...)
			if code != tt.wantCode || stdout != "" || !strings.Contains(stderr, tt.wantErrHas) {
				t.Errorf("run(%q) = (%d, %q, %q), want code %d, empty stdout, stderr containing %q",
					tt.args, code, stdout, stderr, tt.wantCode, tt.wantErrHas)
			}
		})
	}
}
