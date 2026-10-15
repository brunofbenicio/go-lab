// Package todoapi carries the OpenAPI document of the todo API so the server
// can ship it inside the binary.
package todoapi

import _ "embed"

// Spec is the OpenAPI 3 description of the API, in YAML.
//
//go:embed openapi.yaml
var Spec []byte
