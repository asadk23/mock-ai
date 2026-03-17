package middleware_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	"github.com/asadk23/mock-ai/internal/middleware"
)

func TestLogging_StatusOK(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	handler := middleware.Logging(&logger)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	logOutput := buf.String()
	assert.Contains(t, logOutput, `"method":"GET"`)
	assert.Contains(t, logOutput, `"path":"/v1/chat/completions"`)
	assert.Contains(t, logOutput, `"status":200`)
	assert.Contains(t, logOutput, `"level":"info"`)
}

func TestLogging_Status4xx(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	errHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	})

	handler := middleware.Logging(&logger)(errHandler)

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)

	logOutput := buf.String()
	assert.Contains(t, logOutput, `"status":400`)
	assert.Contains(t, logOutput, `"level":"warn"`)
}

func TestLogging_Status5xx(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	errHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	handler := middleware.Logging(&logger)(errHandler)

	req := httptest.NewRequest(http.MethodGet, "/health", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	logOutput := buf.String()
	assert.Contains(t, logOutput, `"status":500`)
	assert.Contains(t, logOutput, `"level":"error"`)
}

func TestLogging_Duration(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	handler := middleware.Logging(&logger)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	logOutput := buf.String()
	assert.Contains(t, logOutput, `"duration"`)
}
