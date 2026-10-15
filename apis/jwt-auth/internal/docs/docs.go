// Package docs serves the OpenAPI document of the API and an interactive
// Swagger UI page that renders it.
//
// Routes:
//
//	GET /openapi.yaml  the raw OpenAPI document
//	GET /docs          Swagger UI, loaded from a CDN, pointing at /openapi.yaml
//
// The package does not embed the document itself: the caller passes the bytes
// in, which keeps the package reusable and easy to test.
package docs

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"net/http"
)

// specPath is where the OpenAPI document is served and where Swagger UI loads
// it from.
const specPath = "/openapi.yaml"

// pageTemplate is the Swagger UI shell. The assets come from a pinned version
// of swagger-ui-dist on a public CDN, so the page needs Internet access in the
// browser, but the server needs nothing beyond the document itself.
var pageTemplate = template.Must(template.New("docs").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Title}}</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14/swagger-ui-bundle.js"></script>
  <script>
    window.addEventListener("load", function () {
      window.ui = SwaggerUIBundle({
        url: {{.SpecURL}},
        dom_id: "#swagger-ui"
      });
    });
  </script>
</body>
</html>
`))

// Docs serves the documentation routes. Create it with New and mount it with
// Register.
type Docs struct {
	spec []byte
	page []byte
}

// New builds a Docs for the given page title and OpenAPI document. It fails if
// the document is empty or the page cannot be rendered.
func New(title string, spec []byte) (*Docs, error) {
	if len(spec) == 0 {
		return nil, errors.New("docs: the OpenAPI document is empty")
	}
	var page bytes.Buffer
	data := struct{ Title, SpecURL string }{Title: title, SpecURL: specPath}
	if err := pageTemplate.Execute(&page, data); err != nil {
		return nil, fmt.Errorf("docs: render page: %w", err)
	}
	return &Docs{spec: spec, page: page.Bytes()}, nil
}

// route couples a ServeMux pattern with its handler method, so Routes and
// Register share a single source of truth.
type route struct {
	pattern string
	handle  func(*Docs, http.ResponseWriter, *http.Request)
}

var routes = []route{
	{"GET " + specPath, (*Docs).serveSpec},
	{"GET /docs", (*Docs).serveUI},
}

// Routes lists the ServeMux patterns registered by Register. It is used to
// check that every route is documented in the OpenAPI specification.
func Routes() []string {
	patterns := make([]string, len(routes))
	for i, r := range routes {
		patterns[i] = r.pattern
	}
	return patterns
}

// Register mounts the documentation routes on mux.
func (d *Docs) Register(mux *http.ServeMux) {
	for _, r := range routes {
		mux.HandleFunc(r.pattern, func(w http.ResponseWriter, req *http.Request) {
			r.handle(d, w, req)
		})
	}
}

func (d *Docs) serveSpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(d.spec)
}

func (d *Docs) serveUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(d.page)
}
