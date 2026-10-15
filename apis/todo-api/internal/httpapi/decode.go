package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
)

// maxBodyBytes caps the size of a request body (1 MiB).
const maxBodyBytes = 1 << 20

var (
	errUnsupportedMediaType = errors.New("content type must be application/json")
	errTrailingData         = errors.New("request body must contain a single JSON value")
)

// decodeJSON reads one JSON value from the request body into dst. It rejects
// unknown fields, trailing data, oversized bodies and non-JSON content types.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	if ct := r.Header.Get("Content-Type"); ct != "" {
		mediaType, _, err := mime.ParseMediaType(ct)
		if err != nil || mediaType != "application/json" {
			return errUnsupportedMediaType
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return err
		}
		return errTrailingData
	}
	return nil
}

// writeDecodeError turns an error from decodeJSON into the matching response.
func writeDecodeError(w http.ResponseWriter, err error) {
	var (
		maxErr *http.MaxBytesError
		syntax *json.SyntaxError
		typ    *json.UnmarshalTypeError
	)

	switch {
	case errors.Is(err, errUnsupportedMediaType):
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", err.Error())
	case errors.As(err, &maxErr):
		writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large",
			fmt.Sprintf("request body must be at most %d bytes", maxErr.Limit))
	case errors.Is(err, io.EOF):
		writeError(w, http.StatusBadRequest, "invalid_json", "request body must not be empty")
	case errors.As(err, &syntax):
		writeError(w, http.StatusBadRequest, "invalid_json",
			fmt.Sprintf("malformed JSON at position %d", syntax.Offset))
	case errors.Is(err, io.ErrUnexpectedEOF):
		writeError(w, http.StatusBadRequest, "invalid_json", "malformed JSON: unexpected end of body")
	case errors.As(err, &typ):
		writeError(w, http.StatusBadRequest, "invalid_json",
			fmt.Sprintf("field %q must be of type %s", typ.Field, typ.Type))
	default:
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
	}
}
