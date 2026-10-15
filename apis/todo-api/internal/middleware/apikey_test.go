package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAPIKey(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		headers    map[string]string
		wantStatus int
	}{
		{name: "reads need no key", method: http.MethodGet, wantStatus: http.StatusOK},
		{name: "head needs no key", method: http.MethodHead, wantStatus: http.StatusOK},
		{name: "options needs no key", method: http.MethodOptions, wantStatus: http.StatusOK},
		{name: "write without key", method: http.MethodPost, wantStatus: http.StatusUnauthorized},
		{name: "write with wrong key", method: http.MethodPost, headers: map[string]string{"X-API-Key": "nope"}, wantStatus: http.StatusUnauthorized},
		{name: "write with X-API-Key", method: http.MethodPost, headers: map[string]string{"X-API-Key": "secret-one"}, wantStatus: http.StatusOK},
		{name: "second configured key works", method: http.MethodDelete, headers: map[string]string{"X-API-Key": "secret-two"}, wantStatus: http.StatusOK},
		{name: "bearer token", method: http.MethodPut, headers: map[string]string{"Authorization": "Bearer secret-one"}, wantStatus: http.StatusOK},
		{name: "bearer scheme is case insensitive", method: http.MethodPatch, headers: map[string]string{"Authorization": "bearer secret-two"}, wantStatus: http.StatusOK},
		{name: "wrong bearer token", method: http.MethodPut, headers: map[string]string{"Authorization": "Bearer wrong"}, wantStatus: http.StatusUnauthorized},
		{name: "other auth scheme is ignored", method: http.MethodPost, headers: map[string]string{"Authorization": "Basic c2VjcmV0LW9uZQ=="}, wantStatus: http.StatusUnauthorized},
		{name: "prefix of a key is not enough", method: http.MethodPost, headers: map[string]string{"X-API-Key": "secret"}, wantStatus: http.StatusUnauthorized},
		{name: "empty bearer token", method: http.MethodPost, headers: map[string]string{"Authorization": "Bearer "}, wantStatus: http.StatusUnauthorized},
	}

	h := RequireAPIKey("secret-one", "secret-two")(okHandler())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/todos", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantStatus == http.StatusUnauthorized {
				if rec.Header().Get("WWW-Authenticate") == "" {
					t.Error("401 responses must carry WWW-Authenticate")
				}
				if got := rec.Header().Get("Content-Type"); got != "application/json" {
					t.Errorf("Content-Type = %q, want application/json", got)
				}
			}
		})
	}
}

func TestRequireAPIKeyDisabledWithoutKeys(t *testing.T) {
	for _, keys := range [][]string{nil, {}, {""}, {"  "}} {
		h := RequireAPIKey(keys...)(okHandler())
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/todos/1", nil))

		if rec.Code != http.StatusOK {
			t.Errorf("keys %q: status = %d, want 200 because no key is configured", keys, rec.Code)
		}
	}
}
