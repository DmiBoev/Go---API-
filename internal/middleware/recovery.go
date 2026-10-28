package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
)

func RecoveryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wrapped := capture(w)
			defer func() {
				if recovered := recover(); recovered != nil {
					if recovered == http.ErrAbortHandler {
						panic(recovered)
					}
					logger.Error("request panic", "method", r.Method, "path", r.URL.Path,
						"panic", recovered, "stack", string(debug.Stack()))
					// A response already sent cannot be replaced with a 500. Abort the
					// connection rather than append an error to a successful partial body.
					if wrapped.status != 0 {
						panic(http.ErrAbortHandler)
					}
					wrapped.Header().Del("Content-Length")
					wrapped.Header().Set("Content-Type", "application/json")
					wrapped.WriteHeader(http.StatusInternalServerError)
					_ = json.NewEncoder(wrapped).Encode(map[string]string{"error": "Internal server error"})
				}
			}()
			next.ServeHTTP(wrapped, r)
		})
	}
}
