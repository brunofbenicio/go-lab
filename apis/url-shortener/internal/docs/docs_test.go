package docs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testSpec = "openapi: 3.0.3\ninfo:\n  title: Test\n  version: 1.0.0\npaths: {}\n"

func newMux(t *testing.T, title string) *http.ServeMux {
	t.Helper()
	d, err := New(title, []byte(testSpec))
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	mux := http.NewServeMux()
	d.Register(mux)
	return mux
}

func TestNewRejectsEmptySpec(t *testing.T) {
	for name, spec := range map[string][]byte{"nil": nil, "empty": {}} {
		t.Run(name, func(t *testing.T) {
			if _, err := New("API", spec); err == nil {
				t.Error("New() accepted an empty document")
			}
		})
	}
}

func TestRoutesAreRegistered(t *testing.T) {
	mux := newMux(t, "API")
	for _, pattern := range Routes() {
		method, path, _ := strings.Cut(pattern, " ")
		req := httptest.NewRequest(method, path, nil)
		if _, got := mux.Handler(req); got != pattern {
			t.Errorf("%s %s matched pattern %q, want %q", method, path, got, pattern)
		}
	}
}

func TestServeSpec(t *testing.T) {
	rec := httptest.NewRecorder()
	newMux(t, "API").ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/yaml" {
		t.Errorf("Content-Type = %q, want application/yaml", ct)
	}
	if rec.Body.String() != testSpec {
		t.Errorf("body differs from the document passed to New:\n%s", rec.Body.String())
	}
}

func TestServeUI(t *testing.T) {
	tests := []struct {
		name      string
		title     string
		want      []string
		wantAbsen []string
	}{
		{
			name:  "points Swagger UI at the pinned CDN version and the spec",
			title: "Links API",
			want: []string{
				"<title>Links API</title>",
				"https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui.css",
				"https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui-bundle.js",
				`url: "/openapi.yaml"`,
				`id="swagger-ui"`,
			},
		},
		{
			name:      "escapes the title",
			title:     `<script>alert("x")</script>`,
			want:      []string{"<title>&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt;</title>"},
			wantAbsen: []string{`<script>alert("x")</script>`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newMux(t, tt.title).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/docs", nil))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
				t.Errorf("Content-Type = %q, want text/html; charset=utf-8", ct)
			}
			body := rec.Body.String()
			for _, s := range tt.want {
				if !strings.Contains(body, s) {
					t.Errorf("page does not contain %q\n%s", s, body)
				}
			}
			for _, s := range tt.wantAbsen {
				if strings.Contains(body, s) {
					t.Errorf("page must not contain %q", s)
				}
			}
		})
	}
}

func TestAllowedMethods(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   int
	}{
		{http.MethodHead, "/docs", http.StatusOK},
		{http.MethodHead, "/openapi.yaml", http.StatusOK},
		{http.MethodPost, "/docs", http.StatusMethodNotAllowed},
		{http.MethodPut, "/openapi.yaml", http.StatusMethodNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			newMux(t, "API").ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
