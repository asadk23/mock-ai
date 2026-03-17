package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

const (
	// _openAIVersion matches the version header returned by the real OpenAI API.
	_openAIVersion = "2020-10-01"

	// _openAIOrganization is a hardcoded org ID for mock responses.
	_openAIOrganization = "org-mock-ai"
)

// deferredTimingWriter wraps http.ResponseWriter to inject the
// openai-processing-ms header just before the status line is written.
// This is necessary because headers set after WriteHeader is called
// are silently ignored by net/http.
type deferredTimingWriter struct {
	http.ResponseWriter
	start       time.Time
	wroteHeader bool
}

func (dtw *deferredTimingWriter) WriteHeader(code int) {
	if !dtw.wroteHeader {
		dtw.wroteHeader = true
		elapsed := time.Since(dtw.start).Milliseconds()
		dtw.ResponseWriter.Header().Set("openai-processing-ms", strconv.FormatInt(elapsed, 10))
	}

	dtw.ResponseWriter.WriteHeader(code)
}

// Write ensures WriteHeader is called (with 200) before the first write,
// matching net/http's implicit behavior but allowing us to inject the
// processing-ms header.
func (dtw *deferredTimingWriter) Write(b []byte) (int, error) {
	if !dtw.wroteHeader {
		dtw.WriteHeader(http.StatusOK)
	}

	return dtw.ResponseWriter.Write(b)
}

// Flush implements http.Flusher by delegating to the underlying ResponseWriter.
// Required for SSE streaming to work through the middleware chain.
func (dtw *deferredTimingWriter) Flush() {
	if flusher, ok := dtw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// OpenAIHeaders returns middleware that adds OpenAI-compatible response headers
// to every response:
//   - x-request-id: unique UUID per request
//   - openai-organization: hardcoded org ID
//   - openai-processing-ms: elapsed time in milliseconds
//   - openai-version: API version string
func OpenAIHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()
		w.Header().Set("x-request-id", requestID)
		w.Header().Set("openai-organization", _openAIOrganization)
		w.Header().Set("openai-version", _openAIVersion)

		dtw := &deferredTimingWriter{
			ResponseWriter: w,
			start:          time.Now(),
		}

		next.ServeHTTP(dtw, r)
	})
}
