package httpapi

import "net/http"

// handleHealth reports that the process is up and able to serve requests.
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
