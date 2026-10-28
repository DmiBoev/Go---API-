package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

type responseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *responseWriter) WriteHeader(status int) {
	if status >= 100 && status < 200 {
		w.ResponseWriter.WriteHeader(status)
		return
	}
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(body)
	w.bytes += n
	return n, err
}

func capture(w http.ResponseWriter) *responseWriter {
	if wrapped, ok := w.(*responseWriter); ok {
		return wrapped
	}
	return &responseWriter{ResponseWriter: w}
}

func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := capture(w)
			defer func() {
				status := wrapped.status
				if status == 0 {
					status = http.StatusOK
				}
				logger.Info("request", "method", r.Method, "path", r.URL.Path,
					"status", status, "bytes", wrapped.bytes, "duration_ms", float64(time.Since(start).Microseconds())/1000)
			}()
			next.ServeHTTP(wrapped, r)
		})
	}
}

// Wrap records requests, recovers handler panics, and handles CORS preflight.
func Wrap(logger *slog.Logger, origins []string, next http.Handler) http.Handler {
	return LoggingMiddleware(logger)(RecoveryMiddleware(logger)(CORSMiddleware(origins)(next)))
}
