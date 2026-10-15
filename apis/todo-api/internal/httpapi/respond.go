package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
)

// errorBody is the envelope of every error response:
//
//	{"error": {"code": "validation_failed", "message": "...", "details": [...]}}
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Details []fieldError `json:"details,omitempty"`
}

// fieldError points at one invalid field of a request.
type fieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// writeJSON encodes v as the response body with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// The status line is already sent, so all we can do is record it.
		log.Printf("httpapi: encoding response: %v", err)
	}
}

// writeError sends the standard error envelope.
func writeError(w http.ResponseWriter, status int, code, message string, details ...fieldError) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message, Details: details}})
}
