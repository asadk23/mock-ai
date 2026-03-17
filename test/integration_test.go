// Package test contains integration tests for the mock-ai server.
// These tests exercise the full HTTP stack including middleware, routing,
// and handler logic via httptest.NewServer.
package test

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asadk23/mock-ai/internal/fixture"
	"github.com/asadk23/mock-ai/internal/model"
	"github.com/asadk23/mock-ai/internal/server"
	"github.com/asadk23/mock-ai/internal/store"
)

const (
	testAuthToken = "test-secret-token"
)

// newTestServer creates a fully-wired test server with auth enabled.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	logger := zerolog.New(io.Discard)
	memStore := store.NewMemory()
	fixtures := fixture.Default()

	r := server.NewRouter(server.Options{
		Store:       memStore,
		Fixtures:    fixtures,
		Logger:      &logger,
		AuthEnabled: true,
		AuthToken:   testAuthToken,
	})

	return httptest.NewServer(r)
}

// newTestServerNoAuth creates a fully-wired test server with auth disabled.
func newTestServerNoAuth(t *testing.T) *httptest.Server {
	t.Helper()

	logger := zerolog.New(io.Discard)
	memStore := store.NewMemory()
	fixtures := fixture.Default()

	r := server.NewRouter(server.Options{
		Store:       memStore,
		Fixtures:    fixtures,
		Logger:      &logger,
		AuthEnabled: false,
	})

	return httptest.NewServer(r)
}

// doJSON performs an authenticated JSON request and returns the response.
func doJSON(t *testing.T, method, url string, body any) *http.Response {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, reqBody)
	require.NoError(t, err)

	req.Header.Set("Authorization", "Bearer "+testAuthToken)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	return resp
}

// --- Health ---

func TestHealth(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp := doJSON(t, http.MethodGet, ts.URL+"/health", nil)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]string
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "ok", result["status"])
}

// --- Auth Middleware ---

func TestAuth_MissingToken(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/health", http.NoBody)
	require.NoError(t, err)
	// No Authorization header.

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	var apiErr model.Error
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&apiErr))
	assert.Equal(t, "invalid_api_key", apiErr.Error.Code)
}

func TestAuth_InvalidToken(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/health", http.NoBody)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer wrong-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuth_Disabled(t *testing.T) {
	ts := newTestServerNoAuth(t)
	defer ts.Close()

	// Request with no auth header should succeed when auth is disabled.
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/health", http.NoBody)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// --- OpenAI Headers Middleware ---

func TestOpenAIHeaders_Present(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp := doJSON(t, http.MethodGet, ts.URL+"/health", nil)
	defer resp.Body.Close()

	assert.NotEmpty(t, resp.Header.Get("x-request-id"))
	assert.NotEmpty(t, resp.Header.Get("openai-organization"))
	assert.NotEmpty(t, resp.Header.Get("openai-version"))
}

// --- Chat Completions (full lifecycle) ---

func TestChatCompletionCreate(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	body := map[string]any{
		"model":    "gpt-4o",
		"messages": []map[string]string{{"role": "user", "content": "Hello"}},
	}

	resp := doJSON(t, http.MethodPost, ts.URL+"/v1/chat/completions", body)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var completion model.ChatCompletion
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&completion))

	assert.NotEmpty(t, completion.ID)
	assert.Equal(t, "chat.completion", completion.Object)
	assert.Equal(t, "gpt-4o", completion.Model)
	require.Len(t, completion.Choices, 1)
	assert.Equal(t, "assistant", completion.Choices[0].Message.Role)
	assert.NotEmpty(t, completion.Choices[0].Message.Content)
}

func TestChatCompletionCreate_Streaming(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	stream := true
	body := map[string]any{
		"model":    "gpt-4o",
		"messages": []map[string]string{{"role": "user", "content": "Hello"}},
		"stream":   stream,
	}

	resp := doJSON(t, http.MethodPost, ts.URL+"/v1/chat/completions", body)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	// Read all SSE data lines.
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	lines := strings.Split(string(data), "\n")
	var dataLines []string
	for _, line := range lines {
		if strings.HasPrefix(line, "data: ") {
			dataLines = append(dataLines, strings.TrimPrefix(line, "data: "))
		}
	}

	require.NotEmpty(t, dataLines)
	// Last data line should be [DONE].
	assert.Equal(t, "[DONE]", dataLines[len(dataLines)-1])

	// First data line should be a valid ChatCompletionChunk.
	var chunk model.ChatCompletionChunk
	require.NoError(t, json.Unmarshal([]byte(dataLines[0]), &chunk))
	assert.Equal(t, "chat.completion.chunk", chunk.Object)
}

func TestChatCompletionCreate_MissingModel(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	body := map[string]any{
		"messages": []map[string]string{{"role": "user", "content": "Hello"}},
	}

	resp := doJSON(t, http.MethodPost, ts.URL+"/v1/chat/completions", body)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	var apiErr model.Error
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&apiErr))
	assert.Equal(t, "invalid_request_error", apiErr.Error.Type)
}

func TestChatCompletion_CRUD(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Create.
	createBody := map[string]any{
		"model":    "gpt-4o",
		"messages": []map[string]string{{"role": "user", "content": "Hello"}},
	}
	resp := doJSON(t, http.MethodPost, ts.URL+"/v1/chat/completions", createBody)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var created model.ChatCompletion
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))

	// Get.
	getResp := doJSON(t, http.MethodGet, ts.URL+"/v1/chat/completions/"+created.ID, nil)
	defer getResp.Body.Close()
	require.Equal(t, http.StatusOK, getResp.StatusCode)

	var fetched model.ChatCompletion
	require.NoError(t, json.NewDecoder(getResp.Body).Decode(&fetched))
	assert.Equal(t, created.ID, fetched.ID)

	// List.
	listResp := doJSON(t, http.MethodGet, ts.URL+"/v1/chat/completions", nil)
	defer listResp.Body.Close()
	require.Equal(t, http.StatusOK, listResp.StatusCode)

	var listResult model.ListResponse
	require.NoError(t, json.NewDecoder(listResp.Body).Decode(&listResult))

	var completions []model.ChatCompletion
	require.NoError(t, json.Unmarshal(listResult.Data, &completions))
	assert.GreaterOrEqual(t, len(completions), 1)

	// Update (metadata).
	updateBody := map[string]any{
		"metadata": map[string]string{"key": "value"},
	}
	updateResp := doJSON(t, http.MethodPost, ts.URL+"/v1/chat/completions/"+created.ID, updateBody)
	defer updateResp.Body.Close()
	require.Equal(t, http.StatusOK, updateResp.StatusCode)

	// Delete.
	deleteResp := doJSON(t, http.MethodDelete, ts.URL+"/v1/chat/completions/"+created.ID, nil)
	defer deleteResp.Body.Close()
	require.Equal(t, http.StatusOK, deleteResp.StatusCode)

	var deleteResult model.DeleteResponse
	require.NoError(t, json.NewDecoder(deleteResp.Body).Decode(&deleteResult))
	assert.True(t, deleteResult.Deleted)

	// Get after delete should 404.
	getAfterDelete := doJSON(t, http.MethodGet, ts.URL+"/v1/chat/completions/"+created.ID, nil)
	defer getAfterDelete.Body.Close()
	assert.Equal(t, http.StatusNotFound, getAfterDelete.StatusCode)
}

func TestChatCompletion_ListMessages(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Create a completion first.
	createBody := map[string]any{
		"model":    "gpt-4o",
		"messages": []map[string]string{{"role": "user", "content": "Hello"}},
	}
	resp := doJSON(t, http.MethodPost, ts.URL+"/v1/chat/completions", createBody)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var created model.ChatCompletion
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))

	// List messages.
	msgResp := doJSON(t, http.MethodGet, ts.URL+"/v1/chat/completions/"+created.ID+"/messages", nil)
	defer msgResp.Body.Close()
	require.Equal(t, http.StatusOK, msgResp.StatusCode)
}

// --- Responses API (full lifecycle) ---

func TestResponseCreate(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	body := map[string]any{
		"model": "gpt-4o",
		"input": "What is 2+2?",
	}

	resp := doJSON(t, http.MethodPost, ts.URL+"/v1/responses", body)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var response model.Response
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&response))

	assert.NotEmpty(t, response.ID)
	assert.Equal(t, "response", response.Object)
	assert.Equal(t, "completed", response.Status)
	assert.Equal(t, "gpt-4o", response.Model)
}

func TestResponseCreate_Streaming(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	body := map[string]any{
		"model":  "gpt-4o",
		"input":  "What is 2+2?",
		"stream": true,
	}

	resp := doJSON(t, http.MethodPost, ts.URL+"/v1/responses", body)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	bodyStr := string(data)
	// Responses API uses named events.
	assert.Contains(t, bodyStr, "event: response.created")
	assert.Contains(t, bodyStr, "event: response.completed")
	assert.Contains(t, bodyStr, "data: [DONE]")
}

func TestResponseCreate_MissingModel(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	body := map[string]any{
		"input": "What is 2+2?",
	}

	resp := doJSON(t, http.MethodPost, ts.URL+"/v1/responses", body)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestResponse_CRUD(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Create.
	createBody := map[string]any{
		"model": "gpt-4o",
		"input": "Hello",
	}
	resp := doJSON(t, http.MethodPost, ts.URL+"/v1/responses", createBody)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var created model.Response
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))

	// Get.
	getResp := doJSON(t, http.MethodGet, ts.URL+"/v1/responses/"+created.ID, nil)
	defer getResp.Body.Close()
	require.Equal(t, http.StatusOK, getResp.StatusCode)

	var fetched model.Response
	require.NoError(t, json.NewDecoder(getResp.Body).Decode(&fetched))
	assert.Equal(t, created.ID, fetched.ID)

	// Delete.
	deleteResp := doJSON(t, http.MethodDelete, ts.URL+"/v1/responses/"+created.ID, nil)
	defer deleteResp.Body.Close()
	require.Equal(t, http.StatusOK, deleteResp.StatusCode)

	// Get after delete should 404.
	getAfterDelete := doJSON(t, http.MethodGet, ts.URL+"/v1/responses/"+created.ID, nil)
	defer getAfterDelete.Body.Close()
	assert.Equal(t, http.StatusNotFound, getAfterDelete.StatusCode)
}

func TestResponse_Cancel(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Create.
	createBody := map[string]any{
		"model": "gpt-4o",
		"input": "Hello",
	}
	resp := doJSON(t, http.MethodPost, ts.URL+"/v1/responses", createBody)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var created model.Response
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))

	// Cancel.
	cancelResp := doJSON(t, http.MethodPost, ts.URL+"/v1/responses/"+created.ID+"/cancel", nil)
	defer cancelResp.Body.Close()
	require.Equal(t, http.StatusOK, cancelResp.StatusCode)

	var canceled model.Response
	require.NoError(t, json.NewDecoder(cancelResp.Body).Decode(&canceled))
	assert.Equal(t, "canceled", canceled.Status)
}

func TestResponse_ListInputItems(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Create.
	createBody := map[string]any{
		"model": "gpt-4o",
		"input": "Hello",
	}
	resp := doJSON(t, http.MethodPost, ts.URL+"/v1/responses", createBody)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var created model.Response
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&created))

	// List input items.
	itemsResp := doJSON(t, http.MethodGet, ts.URL+"/v1/responses/"+created.ID+"/input_items", nil)
	defer itemsResp.Body.Close()
	require.Equal(t, http.StatusOK, itemsResp.StatusCode)
}

func TestResponse_CountTokens(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	body := map[string]any{
		"model": "gpt-4o",
		"input": "Hello world",
	}

	resp := doJSON(t, http.MethodPost, ts.URL+"/v1/responses/count_tokens", body)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result model.CountTokensResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Positive(t, result.TotalTokens)
}

// --- Audio ---

func TestAudio_Speech(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	body := map[string]any{
		"model": "tts-1",
		"input": "Hello, world!",
		"voice": "alloy",
	}

	resp := doJSON(t, http.MethodPost, ts.URL+"/v1/audio/speech", body)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "audio/mpeg", resp.Header.Get("Content-Type"))

	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.NotEmpty(t, data)
}

func TestAudio_Transcription(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	body, contentType := makeMultipartForm(t, "whisper-1", "test.mp3", []byte("fake audio data"))

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/audio/transcriptions", body)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+testAuthToken)
	req.Header.Set("Content-Type", contentType)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result model.AudioTranscriptionResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.NotEmpty(t, result.Text)
}

func TestAudio_Translation(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	body, contentType := makeMultipartForm(t, "whisper-1", "test.mp3", []byte("fake audio data"))

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/v1/audio/translations", body)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+testAuthToken)
	req.Header.Set("Content-Type", contentType)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	var result model.AudioTranslationResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.NotEmpty(t, result.Text)
}

func TestAudio_Speech_MissingFields(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Missing all required fields.
	body := map[string]any{}

	resp := doJSON(t, http.MethodPost, ts.URL+"/v1/audio/speech", body)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// --- Cross-cutting: 404 for unknown routes ---

func TestNotFound(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp := doJSON(t, http.MethodGet, ts.URL+"/v1/nonexistent", nil)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// --- Helpers ---

// makeMultipartForm creates a multipart form body with a file and model field.
func makeMultipartForm(t *testing.T, modelName, fileName string, fileContent []byte) (body *bytes.Buffer, contentType string) {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	require.NoError(t, writer.WriteField("model", modelName))

	part, err := writer.CreateFormFile("file", fileName)
	require.NoError(t, err)

	_, err = part.Write(fileContent)
	require.NoError(t, err)

	require.NoError(t, writer.Close())

	return &buf, writer.FormDataContentType()
}
