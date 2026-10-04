package internal

import (
	jsonv1 "encoding/json"
	"encoding/json/v2"
	"errors"
	"net/http"

	"pusher/log"
)

// jsonResponseOptions keeps response bodies byte-compatible with the previous
// encoding/json (v1) output: zero numbers/bools tagged omitempty are still
// omitted, and map keys are sorted.
var jsonResponseOptions = json.JoinOptions(
	jsonv1.OmitEmptyWithLegacySemantics(true),
	json.Deterministic(true),
)

// writeJSON serializes v as the JSON response body with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v, jsonResponseOptions)
	if err != nil {
		log.Logger().Errorf("failed to encode JSON response: %s", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// writeError writes a {"error": msg} JSON response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// writeEmptyJSON writes an empty {} JSON object, as Pusher clients expect on success.
func writeEmptyJSON(w http.ResponseWriter) {
	writeJSON(w, http.StatusOK, struct{}{})
}

// decodeJSON decodes the request body into v.
func decodeJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return errors.New("request body is empty")
	}
	return jsonv1.NewDecoder(r.Body).Decode(v)
}

// middleware wraps an http.Handler with extra behavior.
type middleware func(http.Handler) http.Handler

// chain applies middlewares so that the first one listed runs first.
func chain(h http.Handler, middlewares ...middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// responseRecorder captures the status code and body size for logging.
type responseRecorder struct {
	http.ResponseWriter
	status int
	size   int
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{ResponseWriter: w}
}

func (r *responseRecorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.size += n
	return n, err
}

// Status returns the response status, defaulting to 200 when nothing was written.
func (r *responseRecorder) Status() int {
	if r.status == 0 {
		return http.StatusOK
	}
	return r.status
}

// Unwrap lets http.ResponseController reach the underlying writer, which is
// how the websocket upgrade hijacks the connection through this wrapper.
func (r *responseRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
