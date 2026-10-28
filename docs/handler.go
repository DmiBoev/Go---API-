package docs

import (
	_ "embed"
	"net/http"

	swaggerFiles "github.com/swaggo/files/v2"
)

//go:embed openapi.yaml
var Spec []byte

//go:embed index.html
var index []byte

// Register serves the contract and embedded Swagger UI without external assets.
func Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = w.Write(Spec)
	})
	mux.HandleFunc("GET /docs/{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	})
	mux.Handle("GET /docs/", http.StripPrefix("/docs/", http.FileServer(http.FS(swaggerFiles.FS))))
}
