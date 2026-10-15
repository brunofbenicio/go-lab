package docs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testSpec = "openapi: 3.0.3\ninfo:\n  title: Test\n"

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	Register(mux, []byte(testSpec))
	return mux
}

func get(h http.Handler, target string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestServesTheSpec(t *testing.T) {
	rec := get(newMux(), "/openapi.yaml", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != testSpec {
		t.Errorf("body = %q, want the spec verbatim", rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/yaml") {
		t.Errorf("Content-Type = %q, want application/yaml", got)
	}
	if rec.Header().Get("ETag") == "" {
		t.Error("the spec should carry an ETag")
	}
}

func TestSpecSupportsConditionalRequests(t *testing.T) {
	mux := newMux()
	etag := get(mux, "/openapi.yaml", nil).Header().Get("ETag")

	cached := get(mux, "/openapi.yaml", map[string]string{"If-None-Match": etag})
	if cached.Code != http.StatusNotModified {
		t.Errorf("matching ETag: status = %d, want 304", cached.Code)
	}
	if cached.Body.Len() != 0 {
		t.Errorf("a 304 must have no body, got %q", cached.Body.String())
	}

	stale := get(mux, "/openapi.yaml", map[string]string{"If-None-Match": `"something-else"`})
	if stale.Code != http.StatusOK {
		t.Errorf("different ETag: status = %d, want 200", stale.Code)
	}
}

func TestSpecETagChangesWithContent(t *testing.T) {
	a, b := http.NewServeMux(), http.NewServeMux()
	Register(a, []byte("one"))
	Register(b, []byte("two"))

	if get(a, "/openapi.yaml", nil).Header().Get("ETag") == get(b, "/openapi.yaml", nil).Header().Get("ETag") {
		t.Error("different documents must not share an ETag")
	}
}

func TestSwaggerUIPage(t *testing.T) {
	rec := get(newMux(), "/docs", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"swagger-ui-dist@5.17.14/swagger-ui.css",
		"swagger-ui-dist@5.17.14/swagger-ui-bundle.js",
		cssIntegrity,
		jsIntegrity,
		`src="/docs/init.js"`,
		`<div id="swagger-ui">`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page is missing %q", want)
		}
	}
	if strings.Contains(strings.ReplaceAll(body, `src="/docs/init.js"`, ""), "<script>") {
		t.Error("the page must not use inline scripts, the CSP forbids them")
	}

	csp := rec.Header().Get("Content-Security-Policy")
	for _, want := range []string{"default-src 'none'", "script-src 'self' https://cdn.jsdelivr.net", "connect-src 'self'", "frame-ancestors 'none'"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP %q is missing %q", csp, want)
		}
	}
	if strings.Contains(csp, "script-src 'self' 'unsafe-inline'") || strings.Contains(csp, "unsafe-eval") {
		t.Errorf("CSP %q must not allow inline or eval scripts", csp)
	}
}

func TestInitScriptPointsAtTheSpec(t *testing.T) {
	rec := get(newMux(), "/docs/init.js", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/javascript") {
		t.Errorf("Content-Type = %q, want text/javascript", got)
	}
	if !strings.Contains(rec.Body.String(), `url: "/openapi.yaml"`) {
		t.Errorf("init script should load /openapi.yaml, got:\n%s", rec.Body.String())
	}
}

func TestRoutesOnlyAnswerReads(t *testing.T) {
	mux := newMux()
	for _, target := range []string{"/openapi.yaml", "/docs", "/docs/init.js"} {
		req := httptest.NewRequest(http.MethodPost, target, nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("POST %s status = %d, want 405", target, rec.Code)
		}
	}
}
