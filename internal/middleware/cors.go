package middleware

import (
	"net/http"
	"strings"
)

func CORSMiddleware(origins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(origins))
	for _, origin := range origins {
		if origin = strings.TrimSpace(origin); origin != "" {
			allowed[origin] = true
		}
	}
	methods := map[string]bool{"GET": true, "HEAD": true, "POST": true, "PATCH": true, "DELETE": true, "OPTIONS": true}
	headers := map[string]bool{"content-type": true, "accept": true}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !allowed["*"] {
				w.Header().Add("Vary", "Origin")
			}
			origin := r.Header.Get("Origin")
			preflight := r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != ""
			if preflight {
				w.Header().Add("Vary", "Access-Control-Request-Method")
				w.Header().Add("Vary", "Access-Control-Request-Headers")
			}
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}
			if !allowed["*"] && !allowed[origin] {
				if preflight {
					http.Error(w, "Origin not allowed", http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
				return
			}
			if allowed["*"] {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			}
			if preflight {
				if !methods[r.Header.Get("Access-Control-Request-Method")] {
					http.Error(w, "Method not allowed", http.StatusForbidden)
					return
				}
				for _, header := range strings.Split(r.Header.Get("Access-Control-Request-Headers"), ",") {
					if header = strings.ToLower(strings.TrimSpace(header)); header != "" && !headers[header] {
						http.Error(w, "Header not allowed", http.StatusForbidden)
						return
					}
				}
				w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
				w.Header().Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
