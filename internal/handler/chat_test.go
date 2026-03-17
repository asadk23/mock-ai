package handler_test

import (
	"bufio"
	"bytes"
	"encoding/json"
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
	"github.com/asadk23/mock-ai/internal/store"
)

func newTestChatHandler() (*handler.ChatHandler, *store.Memory) {
	s := store.NewMemory()
	f := fixture.Default()
	logger := zerolog.Nop()
	return handler.NewChatHandler(s, f, &logger), s
}

func newTestRouter(h *handler.ChatHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Mount("/v1/chat/completions", h.Routes())
	return r
}

func TestChatHandler_Create_NonStreaming(t *testing.T) {
	h, s := newTestChatHandler()
	router := newTestRouter(h)

	body := `{
		"model": "gpt-4o",
		"messages": [
			{"role": "user", "content": "Hello"}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp model.ChatCompletion
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(resp.ID, "chatcmpl-"))
	assert.Equal(t, "chat.completion", resp.Object)
	assert.Equal(t, "gpt-4o", resp.Model)
	assert.Positive(t, resp.Created)
	assert.NotNil(t, resp.SystemFingerprint)

	// Choices.
	require.Len(t, resp.Choices, 1)
	assert.Equal(t, 0, resp.Choices[0].Index)
	assert.Equal(t, "assistant", resp.Choices[0].Message.Role)
	assert.Equal(t, "stop", resp.Choices[0].FinishReason)
	assert.NotEmpty(t, resp.Choices[0].Message.Content)

	// Usage.
	assert.Positive(t, resp.Usage.PromptTokens)
	assert.Positive(t, resp.Usage.CompletionTokens)
	assert.Equal(t, resp.Usage.PromptTokens+resp.Usage.CompletionTokens, resp.Usage.TotalTokens)

	// Verify stored in memory.
	completions := s.ListChatCompletions()
	require.Len(t, completions, 1)
	assert.Equal(t, resp.ID, completions[0].ID)
}

func TestChatHandler_Create_WithMetadata(t *testing.T) {
	h, _ := newTestChatHandler()
	router := newTestRouter(h)

	body := `{
		"model": "gpt-4o",
		"messages": [{"role": "user", "content": "Hi"}],
		"metadata": {"key": "value", "env": "test"}
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp model.ChatCompletion
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, "value", resp.Metadata["key"])
	assert.Equal(t, "test", resp.Metadata["env"])
}

func TestChatHandler_Create_MissingModel(t *testing.T) {
	h, _ := newTestChatHandler()
	router := newTestRouter(h)

	body := `{
		"messages": [{"role": "user", "content": "Hello"}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp model.Error
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, model.ErrTypeInvalidRequest, resp.Error.Type)
	assert.Equal(t, model.ErrCodeMissingParam, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "model")
	require.NotNil(t, resp.Error.Param)
	assert.Equal(t, "model", *resp.Error.Param)
}

func TestChatHandler_Create_MissingMessages(t *testing.T) {
	h, _ := newTestChatHandler()
	router := newTestRouter(h)

	body := `{"model": "gpt-4o"}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp model.Error
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, model.ErrTypeInvalidRequest, resp.Error.Type)
	assert.Contains(t, resp.Error.Message, "messages")
	require.NotNil(t, resp.Error.Param)
	assert.Equal(t, "messages", *resp.Error.Param)
}

func TestChatHandler_Create_EmptyMessages(t *testing.T) {
	h, _ := newTestChatHandler()
	router := newTestRouter(h)

	body := `{"model": "gpt-4o", "messages": []}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestChatHandler_Create_InvalidJSON(t *testing.T) {
	h, _ := newTestChatHandler()
	router := newTestRouter(h)

	body := `{not valid json}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp model.Error
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, model.ErrTypeInvalidRequest, resp.Error.Type)
	assert.Contains(t, resp.Error.Message, "parse")
}

func TestChatHandler_Create_UsesRequestModel(t *testing.T) {
	h, _ := newTestChatHandler()
	router := newTestRouter(h)

	body := `{
		"model": "gpt-4-turbo",
		"messages": [{"role": "user", "content": "test"}]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp model.ChatCompletion
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	// Model should match the request, not the fixture default.
	assert.Equal(t, "gpt-4-turbo", resp.Model)
}

func TestChatHandler_Create_Streaming(t *testing.T) {
	h, s := newTestChatHandler()
	router := newTestRouter(h)

	body := `{
		"model": "gpt-4o",
		"messages": [{"role": "user", "content": "Hello"}],
		"stream": true
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", w.Header().Get("Connection"))

	// Parse SSE events.
	chunks := parseSSEChunks(t, w.Body.String())

	// Should have: role chunk, content chunk, finish chunk, [DONE].
	require.GreaterOrEqual(t, len(chunks), 3, "expected at least 3 chunks")

	// First chunk: role.
	assert.Equal(t, "assistant", chunks[0].Choices[0].Delta.Role)
	assert.NotNil(t, chunks[0].Choices[0].Delta.Content)
	assert.Empty(t, *chunks[0].Choices[0].Delta.Content)

	// Second chunk: content.
	assert.NotNil(t, chunks[1].Choices[0].Delta.Content)
	assert.NotEmpty(t, *chunks[1].Choices[0].Delta.Content)

	// Third chunk: finish.
	require.NotNil(t, chunks[2].Choices[0].FinishReason)
	assert.Equal(t, "stop", *chunks[2].Choices[0].FinishReason)

	// All chunks share the same ID.
	for _, chunk := range chunks {
		assert.Equal(t, chunks[0].ID, chunk.ID)
		assert.Equal(t, model.ChatCompletionChunkObject, chunk.Object)
		assert.Equal(t, "gpt-4o", chunk.Model)
	}

	// [DONE] sentinel.
	assert.Contains(t, w.Body.String(), "data: [DONE]")

	// No usage chunk (stream_options not set).
	for _, chunk := range chunks {
		assert.Nil(t, chunk.Usage, "usage should not be present without stream_options")
	}

	// Verify stored in memory.
	completions := s.ListChatCompletions()
	require.Len(t, completions, 1)
	assert.Equal(t, chunks[0].ID, completions[0].ID)
}

func TestChatHandler_Create_StreamingWithUsage(t *testing.T) {
	h, _ := newTestChatHandler()
	router := newTestRouter(h)

	body := `{
		"model": "gpt-4o",
		"messages": [{"role": "user", "content": "Hello"}],
		"stream": true,
		"stream_options": {"include_usage": true}
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	chunks := parseSSEChunks(t, w.Body.String())

	// Should have: role, content, finish, usage = 4 chunks.
	require.Len(t, chunks, 4)

	// Last chunk: usage with empty choices.
	usageChunk := chunks[3]
	assert.Empty(t, usageChunk.Choices)
	require.NotNil(t, usageChunk.Usage)
	assert.Positive(t, usageChunk.Usage.PromptTokens)
	assert.Positive(t, usageChunk.Usage.CompletionTokens)
	assert.Equal(t, usageChunk.Usage.PromptTokens+usageChunk.Usage.CompletionTokens,
		usageChunk.Usage.TotalTokens)
}

func TestChatHandler_Create_StreamFalse(t *testing.T) {
	h, _ := newTestChatHandler()
	router := newTestRouter(h)

	body := `{
		"model": "gpt-4o",
		"messages": [{"role": "user", "content": "Hello"}],
		"stream": false
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp model.ChatCompletion
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, "chat.completion", resp.Object)
}

func TestChatHandler_Create_StoresMessages(t *testing.T) {
	h, s := newTestChatHandler()
	router := newTestRouter(h)

	body := `{
		"model": "gpt-4o",
		"messages": [
			{"role": "system", "content": "You are helpful."},
			{"role": "user", "content": "Hello"}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp model.ChatCompletion
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	msgs, err := s.ListChatCompletionMessages(resp.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 2)
	assert.Equal(t, "system", msgs[0].Role)
	assert.Equal(t, "user", msgs[1].Role)
}

func TestChatHandler_Create_UniqueIDs(t *testing.T) {
	h, _ := newTestChatHandler()
	router := newTestRouter(h)

	body := `{
		"model": "gpt-4o",
		"messages": [{"role": "user", "content": "Hello"}]
	}`

	ids := make(map[string]bool, 3)
	for range 3 {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var resp model.ChatCompletion
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

		ids[resp.ID] = true
	}

	assert.Len(t, ids, 3, "all IDs should be unique")
}

func TestChatHandler_Create_EmptyBody(t *testing.T) {
	h, _ := newTestChatHandler()
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(nil))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- List tests ---

func TestChatHandler_List_Empty(t *testing.T) {
	h, _ := newTestChatHandler()
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", http.NoBody)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp model.ListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, "list", resp.Object)
	assert.False(t, resp.HasMore)
	assert.Empty(t, resp.FirstID)
	assert.Empty(t, resp.LastID)

	// Data should be an empty JSON array.
	assert.Equal(t, "[]", string(resp.Data))
}

func TestChatHandler_List_WithCompletions(t *testing.T) {
	h, _ := newTestChatHandler()
	router := newTestRouter(h)

	// Create two completions.
	ids := createCompletions(t, router, 2)

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", http.NoBody)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp model.ListResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, "list", resp.Object)
	assert.False(t, resp.HasMore)
	assert.Equal(t, ids[0], resp.FirstID)
	assert.Equal(t, ids[1], resp.LastID)

	// Parse data array.
	var completions []model.ChatCompletion
	require.NoError(t, json.Unmarshal(resp.Data, &completions))

	require.Len(t, completions, 2)
	assert.Equal(t, ids[0], completions[0].ID)
	assert.Equal(t, ids[1], completions[1].ID)
}

// --- Get tests ---

func TestChatHandler_Get_Success(t *testing.T) {
	h, _ := newTestChatHandler()
	router := newTestRouter(h)

	ids := createCompletions(t, router, 1)

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions/"+ids[0], http.NoBody)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp model.ChatCompletion
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, ids[0], resp.ID)
	assert.Equal(t, "chat.completion", resp.Object)
	assert.Equal(t, "gpt-4o", resp.Model)
}

func TestChatHandler_Get_NotFound(t *testing.T) {
	h, _ := newTestChatHandler()
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions/nonexistent-id", http.NoBody)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp model.Error
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, model.ErrCodeNotFound, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "nonexistent-id")
}

// --- Update tests ---

func TestChatHandler_Update_Success(t *testing.T) {
	h, _ := newTestChatHandler()
	router := newTestRouter(h)

	ids := createCompletions(t, router, 1)

	body := `{"metadata": {"env": "staging", "user": "test-user"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions/"+ids[0], strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp model.ChatCompletion
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, ids[0], resp.ID)
	assert.Equal(t, "staging", resp.Metadata["env"])
	assert.Equal(t, "test-user", resp.Metadata["user"])
}

func TestChatHandler_Update_NotFound(t *testing.T) {
	h, _ := newTestChatHandler()
	router := newTestRouter(h)

	body := `{"metadata": {"key": "value"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions/nonexistent-id", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestChatHandler_Update_InvalidJSON(t *testing.T) {
	h, _ := newTestChatHandler()
	router := newTestRouter(h)

	ids := createCompletions(t, router, 1)

	body := `{not valid}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions/"+ids[0], strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- Delete tests ---

func TestChatHandler_Delete_Success(t *testing.T) {
	h, _ := newTestChatHandler()
	router := newTestRouter(h)

	ids := createCompletions(t, router, 1)

	req := httptest.NewRequest(http.MethodDelete, "/v1/chat/completions/"+ids[0], http.NoBody)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp model.DeleteResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, ids[0], resp.ID)
	assert.Equal(t, "chat.completion.deleted", resp.Object)
	assert.True(t, resp.Deleted)

	// Verify it's actually gone.
	getReq := httptest.NewRequest(http.MethodGet, "/v1/chat/completions/"+ids[0], http.NoBody)
	getW := httptest.NewRecorder()
	router.ServeHTTP(getW, getReq)
	assert.Equal(t, http.StatusNotFound, getW.Code)
}

func TestChatHandler_Delete_NotFound(t *testing.T) {
	h, _ := newTestChatHandler()
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/v1/chat/completions/nonexistent-id", http.NoBody)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp model.Error
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, model.ErrCodeNotFound, resp.Error.Code)
}

// --- ListMessages tests ---

func TestChatHandler_ListMessages_Success(t *testing.T) {
	h, _ := newTestChatHandler()
	router := newTestRouter(h)

	// Create a completion with specific messages.
	body := `{
		"model": "gpt-4o",
		"messages": [
			{"role": "system", "content": "You are helpful."},
			{"role": "user", "content": "Hello there"}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var created model.ChatCompletion
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	// List the messages.
	msgReq := httptest.NewRequest(http.MethodGet, "/v1/chat/completions/"+created.ID+"/messages", http.NoBody)
	msgW := httptest.NewRecorder()
	router.ServeHTTP(msgW, msgReq)

	require.Equal(t, http.StatusOK, msgW.Code)

	var resp model.ListResponse
	require.NoError(t, json.Unmarshal(msgW.Body.Bytes(), &resp))

	assert.Equal(t, "list", resp.Object)
	assert.False(t, resp.HasMore)

	var messages []model.ChatMessage
	require.NoError(t, json.Unmarshal(resp.Data, &messages))

	require.Len(t, messages, 2)
	assert.Equal(t, "system", messages[0].Role)
	assert.Equal(t, "user", messages[1].Role)
}

func TestChatHandler_ListMessages_NotFound(t *testing.T) {
	h, _ := newTestChatHandler()
	router := newTestRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions/nonexistent-id/messages", http.NoBody)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp model.Error
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, model.ErrCodeNotFound, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "nonexistent-id")
}

// --- helpers ---

// createCompletions creates n chat completions via the handler and returns their IDs.
func createCompletions(t *testing.T, router *chi.Mux, n int) []string {
	t.Helper()

	body := `{
		"model": "gpt-4o",
		"messages": [{"role": "user", "content": "Hello"}]
	}`

	ids := make([]string, 0, n)
	for range n {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var resp model.ChatCompletion
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		ids = append(ids, resp.ID)
	}

	return ids
}

// parseSSEChunks extracts ChatCompletionChunk objects from SSE data: lines,
// skipping the [DONE] sentinel.
func parseSSEChunks(t *testing.T, body string) []model.ChatCompletionChunk {
	t.Helper()

	var chunks []model.ChatCompletionChunk
	scanner := bufio.NewScanner(strings.NewReader(body))

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == model.StreamDone {
			continue
		}

		var chunk model.ChatCompletionChunk
		err := json.Unmarshal([]byte(payload), &chunk)
		require.NoError(t, err, "failed to parse chunk: %s", payload)

		chunks = append(chunks, chunk)
	}

	return chunks
}
