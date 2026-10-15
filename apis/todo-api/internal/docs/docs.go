// Package docs serves the OpenAPI document and an interactive Swagger UI page.
package docs

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"
)

// swaggerUIVersion is the swagger-ui-dist release loaded from the CDN. The
// integrity hashes below belong to exactly this version: bump them together.
const swaggerUIVersion = "5.17.14"

const (
	cdnBase = "https://cdn.jsdelivr.net/npm/swagger-ui-dist@" + swaggerUIVersion

	// Subresource Integrity hashes (sha384) of the two CDN files, taken from
	// the swagger-ui-dist npm package. The browser refuses to run a file that
	// does not match, so a compromised CDN cannot inject code into the page.
	cssIntegrity = "sha384-wxLW6kwyHktdDGr6Pv1zgm/VGJh99lfUbzSn6HNHBENZlCN7W602k9VkGdxuFvPn"
	jsIntegrity  = "sha384-wmyclcVGX/WhUkdkATwhaK1X1JtiNrr2EoYJ+diV3vj4v6OC5yCeSu+yW13SYJep"
)

const page = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Todo API - Swagger UI</title>
  <link rel="stylesheet" href="` + cdnBase + `/swagger-ui.css" integrity="` + cssIntegrity + `" crossorigin="anonymous">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="` + cdnBase + `/swagger-ui-bundle.js" integrity="` + jsIntegrity + `" crossorigin="anonymous"></script>
  <script src="/docs/init.js"></script>
</body>
</html>
`

// initScript lives in its own file instead of inline so the page can run with
// a Content-Security-Policy that forbids inline scripts.
const initScript = `window.addEventListener("load", function () {
  window.ui = SwaggerUIBundle({
    url: "/openapi.yaml",
    dom_id: "#swagger-ui",
    deepLinking: true,
    persistAuthorization: true
  });
});
`

// contentSecurityPolicy lets the page load Swagger UI from the CDN and call
// this server, and nothing else.
const contentSecurityPolicy = "default-src 'none'; " +
	"script-src 'self' https://cdn.jsdelivr.net; " +
	"style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; " +
	"img-src 'self' data: https://cdn.jsdelivr.net; " +
	"font-src 'self' data: https://cdn.jsdelivr.net; " +
	"connect-src 'self'; " +
	"base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

// Register adds the documentation routes to mux:
//
//	GET /openapi.yaml   the OpenAPI document
//	GET /docs           Swagger UI
//	GET /docs/init.js   the script that starts Swagger UI
func Register(mux *http.ServeMux, spec []byte) {
	mux.Handle("GET /openapi.yaml", specHandler(spec))
	mux.HandleFunc("GET /docs", uiHandler)
	mux.HandleFunc("GET /docs/init.js", initHandler)
}

// specHandler serves spec with a strong ETag so clients revalidate cheaply.
func specHandler(spec []byte) http.Handler {
	sum := sha256.Sum256(spec)
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("ETag", etag)
		// ServeContent answers If-None-Match with 304 and supports HEAD.
		http.ServeContent(w, r, "openapi.yaml", time.Time{}, bytes.NewReader(spec))
	})
}

func uiHandler(w http.ResponseWriter, _ *http.Request) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Security-Policy", contentSecurityPolicy)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "no-referrer")
	_, _ = w.Write([]byte(page))
}

func initHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write([]byte(initScript))
}
