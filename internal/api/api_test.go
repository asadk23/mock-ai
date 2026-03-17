package api_test

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asadk23/mock-ai/internal/api"
	"github.com/asadk23/mock-ai/internal/model"
)

func TestWriteJSON_Success(t *testing.T) {
	w := httptest.NewRecorder()

	data := map[string]string{"greeting": "hello"}
	api.WriteJSON(w, http.StatusOK, data)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var got map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "hello", got["greeting"])
}

func TestWriteJSON_CustomStatus(t *testing.T) {
	w := httptest.NewRecorder()

	api.WriteJSON(w, http.StatusCreated, map[string]string{"id": "123"})

	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestWriteJSON_EncodingFailure(t *testing.T) {
	w := httptest.NewRecorder()

	// math.NaN() cannot be JSON-encoded, triggering the error path.
	api.WriteJSON(w, http.StatusOK, math.NaN())

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "internal encoding error")
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()

	api.WriteError(w, http.StatusBadRequest, "bad request", model.ErrTypeInvalidRequest, model.ErrCodeInvalidRequest)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var got model.Error
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "bad request", got.Error.Message)
	assert.Equal(t, model.ErrTypeInvalidRequest, got.Error.Type)
	assert.Equal(t, model.ErrCodeInvalidRequest, got.Error.Code)
	assert.Nil(t, got.Error.Param)
}

func TestWriteErrorWithParam(t *testing.T) {
	w := httptest.NewRecorder()

	api.WriteErrorWithParam(w, http.StatusBadRequest,
		"missing field", model.ErrTypeInvalidRequest, model.ErrCodeMissingParam, "model")

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var got model.Error
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "missing field", got.Error.Message)
	assert.Equal(t, model.ErrCodeMissingParam, got.Error.Code)
	require.NotNil(t, got.Error.Param)
	assert.Equal(t, "model", *got.Error.Param)
}
