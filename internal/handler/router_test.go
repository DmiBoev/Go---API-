package handler

import (
	"dept-api/docs"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAPIOperationsAreRegistered(t *testing.T) {
	spec, err := openapi3.NewLoader().LoadFromData(docs.Spec)
	require.NoError(t, err)
	mux := NewRouter(NewDepartmentHandler(nil, nil))
	for path, item := range spec.Paths.Map() {
		for method := range item.Operations() {
			requestPath := strings.ReplaceAll(path, "{id}", "1")
			_, pattern := mux.Handler(httptest.NewRequest(method, requestPath, nil))
			expected := method + " " + path
			if strings.HasSuffix(path, "/") {
				expected += "{$}"
			}
			assert.Equal(t, expected, pattern)
		}
	}
}
