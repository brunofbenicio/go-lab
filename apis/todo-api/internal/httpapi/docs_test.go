package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

func TestRouterServesDocumentation(t *testing.T) {
	h := newTestRouter(t)

	spec := do(t, h, http.MethodGet, "/openapi.yaml", "")
	if spec.Code != http.StatusOK || !strings.HasPrefix(spec.Body.String(), "openapi: 3.0.3") {
		t.Errorf("GET /openapi.yaml = %d %.40q, want the embedded spec", spec.Code, spec.Body.String())
	}

	ui := do(t, h, http.MethodGet, "/docs", "")
	if ui.Code != http.StatusOK || !strings.Contains(ui.Body.String(), "swagger-ui") {
		t.Errorf("GET /docs = %d, want the Swagger UI page", ui.Code)
	}
}
