package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/asadk23/mock-ai/internal/middleware"
)

func TestOpenAIHeaders_RequestID(t *testing.T) {
	handler := middleware.OpenAIHeaders(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	requestID := rec.Header().Get("x-request-id")
	assert.NotEmpty(t, requestID)
	// UUID v4 format: 8-4-4-4-12 hex characters
	assert.Len(t, requestID, 36)
}

func TestOpenAIHeaders_UniqueRequestIDs(t *testing.T) {
	handler := middleware.OpenAIHeaders(okHandler)

	req1 := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	req2 := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	assert.NotEqual(t, rec1.Header().Get("x-request-id"), rec2.Header().Get("x-request-id"))
}

func TestOpenAIHeaders_Organization(t *testing.T) {
	handler := middleware.OpenAIHeaders(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, "org-mock-ai", rec.Header().Get("openai-organization"))
}

func TestOpenAIHeaders_Version(t *testing.T) {
	handler := middleware.OpenAIHeaders(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, "2020-10-01", rec.Header().Get("openai-version"))
}

func TestOpenAIHeaders_ProcessingMs(t *testing.T) {
	handler := middleware.OpenAIHeaders(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	processingMs := rec.Header().Get("openai-processing-ms")
	assert.NotEmpty(t, processingMs)
}

func TestOpenAIHeaders_PreservesStatus(t *testing.T) {
	customHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	handler := middleware.OpenAIHeaders(customHandler)

	req := httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.NotEmpty(t, rec.Header().Get("x-request-id"))
}
