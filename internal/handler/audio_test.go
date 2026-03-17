package handler_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asadk23/mock-ai/internal/fixture"
	"github.com/asadk23/mock-ai/internal/handler"
	"github.com/asadk23/mock-ai/internal/model"
)

func newTestAudioHandler() *handler.AudioHandler {
	f := fixture.Default()
	logger := zerolog.Nop()
	return handler.NewAudioHandler(f, &logger)
}

func newTestAudioRouter(h *handler.AudioHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Mount("/v1/audio", h.Routes())
	return r
}

// --- Speech tests ---

func TestAudioHandler_Speech_Success(t *testing.T) {
	h := newTestAudioHandler()
	router := newTestAudioRouter(h)

	body := `{
		"model": "tts-1",
		"input": "Hello world",
		"voice": "alloy"
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/speech", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "audio/mpeg", w.Header().Get("Content-Type"))
	assert.NotEmpty(t, w.Body.Bytes())
}

func TestAudioHandler_Speech_CustomFormat(t *testing.T) {
	h := newTestAudioHandler()
	router := newTestAudioRouter(h)

	body := `{
		"model": "tts-1",
		"input": "Hello world",
		"voice": "alloy",
		"response_format": "opus"
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/speech", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "audio/opus", w.Header().Get("Content-Type"))
}

func TestAudioHandler_Speech_InvalidFormat(t *testing.T) {
	h := newTestAudioHandler()
	router := newTestAudioRouter(h)

	body := `{
		"model": "tts-1",
		"input": "Hello world",
		"voice": "alloy",
		"response_format": "invalid"
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/speech", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp model.Error
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, model.ErrCodeInvalidValue, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "invalid")
}

func TestAudioHandler_Speech_MissingModel(t *testing.T) {
	h := newTestAudioHandler()
	router := newTestAudioRouter(h)

	body := `{"input": "Hello", "voice": "alloy"}`

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/speech", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp model.Error
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Contains(t, resp.Error.Message, "model")
	require.NotNil(t, resp.Error.Param)
	assert.Equal(t, "model", *resp.Error.Param)
}

func TestAudioHandler_Speech_MissingInput(t *testing.T) {
	h := newTestAudioHandler()
	router := newTestAudioRouter(h)

	body := `{"model": "tts-1", "voice": "alloy"}`

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/speech", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp model.Error
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Contains(t, resp.Error.Message, "input")
}

func TestAudioHandler_Speech_MissingVoice(t *testing.T) {
	h := newTestAudioHandler()
	router := newTestAudioRouter(h)

	body := `{"model": "tts-1", "input": "Hello"}`

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/speech", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp model.Error
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Contains(t, resp.Error.Message, "voice")
}

func TestAudioHandler_Speech_InvalidJSON(t *testing.T) {
	h := newTestAudioHandler()
	router := newTestAudioRouter(h)

	body := `{not valid}`

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/speech", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- Transcription tests ---

func TestAudioHandler_Transcription_Success(t *testing.T) {
	h := newTestAudioHandler()
	router := newTestAudioRouter(h)

	body, contentType := createMultipartForm(t, "whisper-1", "test.mp3", []byte("fake audio data"))

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp model.AudioTranscriptionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.NotEmpty(t, resp.Text)
}

func TestAudioHandler_Transcription_MissingModel(t *testing.T) {
	h := newTestAudioHandler()
	router := newTestAudioRouter(h)

	body, contentType := createMultipartForm(t, "", "test.mp3", []byte("fake audio data"))

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp model.Error
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Contains(t, resp.Error.Message, "model")
}

func TestAudioHandler_Transcription_MissingFile(t *testing.T) {
	h := newTestAudioHandler()
	router := newTestAudioRouter(h)

	// Create multipart form with model but no file.
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	require.NoError(t, writer.WriteField("model", "whisper-1"))
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp model.Error
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Contains(t, resp.Error.Message, "file")
}

func TestAudioHandler_Transcription_NotMultipart(t *testing.T) {
	h := newTestAudioHandler()
	router := newTestAudioRouter(h)

	body := `{"model": "whisper-1"}`

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- Translation tests ---

func TestAudioHandler_Translation_Success(t *testing.T) {
	h := newTestAudioHandler()
	router := newTestAudioRouter(h)

	body, contentType := createMultipartForm(t, "whisper-1", "test.mp3", []byte("fake audio data"))

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/translations", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp model.AudioTranslationResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.NotEmpty(t, resp.Text)
}

func TestAudioHandler_Translation_MissingModel(t *testing.T) {
	h := newTestAudioHandler()
	router := newTestAudioRouter(h)

	body, contentType := createMultipartForm(t, "", "test.mp3", []byte("fake audio data"))

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/translations", body)
	req.Header.Set("Content-Type", contentType)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAudioHandler_Translation_MissingFile(t *testing.T) {
	h := newTestAudioHandler()
	router := newTestAudioRouter(h)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	require.NoError(t, writer.WriteField("model", "whisper-1"))
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/v1/audio/translations", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- helpers ---

// createMultipartForm creates a multipart form body with a file and model field.
func createMultipartForm(t *testing.T, modelName, fileName string, fileContent []byte) (body *bytes.Buffer, contentType string) {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	if modelName != "" {
		require.NoError(t, writer.WriteField("model", modelName))
	}

	part, err := writer.CreateFormFile("file", fileName)
	require.NoError(t, err)

	_, err = part.Write(fileContent)
	require.NoError(t, err)

	require.NoError(t, writer.Close())

	return &buf, writer.FormDataContentType()
}
