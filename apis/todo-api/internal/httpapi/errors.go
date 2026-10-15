package httpapi

import (
	"errors"
	"log"
	"net/http"

	"github.com/brunofbenicio/go-lab/apis/todo-api/internal/todo"
)

// writeValidationError answers 422 with one detail per invalid field.
func writeValidationError(w http.ResponseWriter, err error) {
	var verr *todo.ValidationError
	if !errors.As(err, &verr) {
		writeInternalError(w, err)
		return
	}

	details := make([]fieldError, len(verr.Fields))
	for i, f := range verr.Fields {
		details[i] = fieldError{Field: f.Field, Message: f.Message}
	}
	writeError(w, http.StatusUnprocessableEntity, "validation_failed", "the request body is invalid", details...)
}

// writeInternalError logs the real cause and answers a generic 500, so
// internals never leak to clients.
func writeInternalError(w http.ResponseWriter, err error) {
	log.Printf("httpapi: internal error: %v", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "something went wrong")
}
