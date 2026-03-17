package middleware

import (
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

// statusRecorder wraps http.ResponseWriter to capture the status code.
// It also implements http.Flusher by delegating to the underlying writer,
// which is required for SSE streaming to work through the middleware chain.
type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.statusCode = code
	sr.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher by delegating to the underlying ResponseWriter.
// If the underlying writer does not support flushing, this is a no-op.
func (sr *statusRecorder) Flush() {
	if flusher, ok := sr.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Logging returns middleware that logs each request using zerolog.
// It records the method, path, status code, and duration.
func Logging(logger *zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			rec := &statusRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			next.ServeHTTP(rec, r)

			duration := time.Since(start)

			logRequest(logger, r, rec.statusCode, duration)
		})
	}
}

// logRequest emits a structured log event at the appropriate level based on
// the HTTP status code.
func logRequest(logger *zerolog.Logger, r *http.Request, status int, duration time.Duration) {
	switch {
	case status >= http.StatusInternalServerError:
		logger.Error().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", status).
			Dur("duration", duration).
			Msg("request")
	case status >= http.StatusBadRequest:
		logger.Warn().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", status).
			Dur("duration", duration).
			Msg("request")
	default:
		logger.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", status).
			Dur("duration", duration).
			Msg("request")
	}
}
