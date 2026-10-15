// Package jwtauth holds the OpenAPI document of the JWT authentication API.
//
// The document lives next to this file as openapi.yaml and is embedded in the
// binary, so the server can publish it without reading anything from disk.
// The executable is in cmd/server.
package jwtauth

import (
	"bytes"
	_ "embed"
)

//go:embed openapi.yaml
var openAPI []byte

// OpenAPI returns the OpenAPI 3.0.3 document describing the API, in YAML. The
// returned slice is a copy and may be modified freely.
func OpenAPI() []byte {
	return bytes.Clone(openAPI)
}
