package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoggingStatusAndBytes(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusCreated, http.StatusNotFound} {
		var logs bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&logs, nil))
		h := LoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if status != http.StatusOK {
				w.WriteHeader(status)
			}
			_, _ = w.Write([]byte("test"))
		}))
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, httptest.NewRequest("GET", "/test?secret=excluded", nil))
		var entry map[string]any
		require.NoError(t, json.Unmarshal(logs.Bytes(), &entry))
		assert.Equal(t, float64(status), entry["status"])
		assert.Equal(t, float64(4), entry["bytes"])
		assert.Equal(t, "/test", entry["path"])
		assert.Contains(t, entry, "duration_ms")
	}
}

func TestRecoveryAndLogging(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	calls := 0
	h := Wrap(logger, []string{"https://client.example"}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			panic("private panic detail")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "https://client.example")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.JSONEq(t, `{"error":"Internal server error"}`, rr.Body.String())
	assert.Equal(t, "https://client.example", rr.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, logs.String(), `"status":500`)
	assert.Contains(t, logs.String(), `"stack":`)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusNoContent, rr.Code)
}

func TestRecoveryAfterWriteAbortsResponse(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	h := RecoveryMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("partial"))
		panic("failed after writing")
	}))
	rr := httptest.NewRecorder()
	assert.PanicsWithValue(t, http.ErrAbortHandler, func() { h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil)) })
	assert.Equal(t, "partial", rr.Body.String())
}

func TestCORS(t *testing.T) {
	for _, tc := range []struct {
		name, origin, method, headers string
		origins                       []string
		wantStatus                    int
		wantOrigin                    string
		wantCalled                    bool
	}{
		{name: "allowed preflight", origins: []string{"https://client.example"}, origin: "https://client.example", method: "PATCH", headers: "Content-Type, Accept", wantStatus: 204, wantOrigin: "https://client.example"},
		{name: "denied origin", origins: []string{"https://client.example"}, origin: "https://other.example", method: "POST", wantStatus: 403},
		{name: "denied method", origins: []string{"*"}, origin: "https://client.example", method: "PUT", wantStatus: 403, wantOrigin: "*"},
		{name: "denied header", origins: []string{"*"}, origin: "https://client.example", method: "POST", headers: "Authorization", wantStatus: 403, wantOrigin: "*"},
		{name: "wildcard", origins: []string{"*"}, origin: "https://client.example", method: "POST", wantStatus: 204, wantOrigin: "*"},
		{name: "ordinary request", origins: []string{"https://client.example"}, origin: "https://client.example", wantStatus: 200, wantOrigin: "https://client.example", wantCalled: true},
		{name: "no origin", origins: []string{"https://client.example"}, wantStatus: 200, wantCalled: true},
		{name: "untrusted ordinary request", origins: []string{"https://client.example"}, origin: "https://other.example", wantStatus: 200, wantCalled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			h := CORSMiddleware(tc.origins)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { called = true; w.WriteHeader(200) }))
			method := "GET"
			if tc.method != "" {
				method = "OPTIONS"
			}
			req := httptest.NewRequest(method, "/departments/1", nil)
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Access-Control-Request-Method", tc.method)
			req.Header.Set("Access-Control-Request-Headers", tc.headers)
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, req)
			assert.Equal(t, tc.wantStatus, rr.Code)
			assert.Equal(t, tc.wantOrigin, rr.Header().Get("Access-Control-Allow-Origin"))
			assert.Equal(t, tc.wantCalled, called)
			assert.Empty(t, rr.Header().Get("Access-Control-Allow-Credentials"))
			if tc.wantStatus == 204 {
				assert.Contains(t, rr.Header().Get("Access-Control-Allow-Methods"), tc.method)
			}
		})
	}
}
