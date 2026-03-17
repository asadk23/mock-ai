package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asadk23/mock-ai/internal/middleware"
	"github.com/asadk23/mock-ai/internal/model"
)

// okHandler is a simple handler that writes a 200 OK response.
var okHandler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func TestAuth_Disabled(t *testing.T) {
	handler := middleware.Auth(false, "secret")(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAuth_NoHeader(t *testing.T) {
	handler := middleware.Auth(true, "")(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var apiErr model.Error
	err := json.NewDecoder(rec.Body).Decode(&apiErr)
	require.NoError(t, err)
	assert.Equal(t, model.ErrCodeInvalidAPIKey, apiErr.Error.Code)
	assert.Contains(t, apiErr.Error.Message, "API key")
}

func TestAuth_InvalidFormat(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{name: "no_bearer_prefix", header: "Token sk-123"},
		{name: "bearer_no_token", header: "Bearer "},
		{name: "bearer_only", header: "Bearer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := middleware.Auth(true, "")(okHandler)

			req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			req.Header.Set("Authorization", tt.header)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusUnauthorized, rec.Code)

			var apiErr model.Error
			err := json.NewDecoder(rec.Body).Decode(&apiErr)
			require.NoError(t, err)
			assert.Equal(t, model.ErrCodeInvalidAPIKey, apiErr.Error.Code)
		})
	}
}

func TestAuth_AnyTokenAccepted(t *testing.T) {
	handler := middleware.Auth(true, "")(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer sk-anything")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAuth_TokenMismatch(t *testing.T) {
	handler := middleware.Auth(true, "sk-correct")(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer sk-wrong")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)

	var apiErr model.Error
	err := json.NewDecoder(rec.Body).Decode(&apiErr)
	require.NoError(t, err)
	assert.Equal(t, model.ErrCodeInvalidAPIKey, apiErr.Error.Code)
	assert.Contains(t, apiErr.Error.Message, "sk-***")
}

func TestAuth_TokenMatch(t *testing.T) {
	handler := middleware.Auth(true, "sk-correct")(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer sk-correct")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
