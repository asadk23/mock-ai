package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asadk23/mock-ai/internal/fixture"
	"github.com/asadk23/mock-ai/internal/model"
	"github.com/asadk23/mock-ai/internal/server"
	"github.com/asadk23/mock-ai/internal/store"
)

func newTestRouter(authEnabled bool, token string) http.Handler {
	logger := zerolog.Nop()
	return server.NewRouter(server.Options{
		Store:       store.NewMemory(),
		Fixtures:    fixture.Default(),
		Logger:      &logger,
		AuthEnabled: authEnabled,
		AuthToken:   token,
	})
}

func TestRouter_HealthEndpoint(t *testing.T) {
	r := newTestRouter(false, "")

	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"status":"ok"}`, w.Body.String())
}

func TestRouter_HealthEndpoint_HasOpenAIHeaders(t *testing.T) {
	r := newTestRouter(false, "")

	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Header().Get("x-request-id"))
	assert.NotEmpty(t, w.Header().Get("openai-version"))
	assert.NotEmpty(t, w.Header().Get("openai-processing-ms"))
}

func TestRouter_ChatRoutesMounted(t *testing.T) {
	r := newTestRouter(false, "")

	// List chat completions should return a list response.
	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.ListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "list", resp.Object)
}

func TestRouter_ResponsesRoutesMounted(t *testing.T) {
	r := newTestRouter(false, "")

	// GET a non-existent response should return 404 (proving routes are mounted).
	req := httptest.NewRequest(http.MethodGet, "/v1/responses/nonexistent", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRouter_AudioRoutesMounted(t *testing.T) {
	r := newTestRouter(false, "")

	// POST speech with empty body should return 400 (proving routes are mounted).
	req := httptest.NewRequest(http.MethodPost, "/v1/audio/speech", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRouter_AuthEnabled(t *testing.T) {
	r := newTestRouter(true, "sk-test-token")

	// Request without auth should be rejected.
	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Request with correct auth should succeed.
	req = httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	req.Header.Set("Authorization", "Bearer sk-test-token")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRouter_AuthDisabled(t *testing.T) {
	r := newTestRouter(false, "")

	// Request without auth should succeed when auth is disabled.
	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRouter_UnknownRoute_Returns404(t *testing.T) {
	r := newTestRouter(false, "")

	req := httptest.NewRequest(http.MethodGet, "/v1/nonexistent", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}
