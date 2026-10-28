package docs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAPIValid(t *testing.T) {
	spec, err := openapi3.NewLoader().LoadFromData(Spec)
	require.NoError(t, err)
	require.NoError(t, spec.Validate(context.Background()))
	operations := 0
	for _, item := range spec.Paths.Map() {
		operations += len(item.Operations())
	}
	assert.Equal(t, 5, operations)
}

func TestDocumentationServedWithLocalAssets(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)
	for _, tc := range []struct{ path, contentType, contains string }{
		{"/openapi.yaml", "application/yaml", "openapi: 3.0.3"},
		{"/docs/", "text/html", `url: "/openapi.yaml"`},
		{"/docs/swagger-ui.css", "text/css", "swagger-ui"},
		{"/docs/swagger-ui-bundle.js", "javascript", "SwaggerUIBundle"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			rr := httptest.NewRecorder()
			mux.ServeHTTP(rr, httptest.NewRequest("GET", tc.path, nil))
			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Contains(t, rr.Header().Get("Content-Type"), tc.contentType)
			assert.Contains(t, rr.Body.String(), tc.contains)
		})
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest("GET", "/docs", nil))
	assert.Equal(t, http.StatusMovedPermanently, rr.Code)
	assert.Equal(t, "/docs/", rr.Header().Get("Location"))
}
