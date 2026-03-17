package handler_test

import (
	"bufio"
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

func newTestResponseHandler() (*handler.ResponseHandler, *store.Memory) {
	s := store.NewMemory()
	f := fixture.Default()
	logger := zerolog.Nop()
	return handler.NewResponseHandler(s, f, &logger), s
}

func newTestResponseRouter(h *handler.ResponseHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Mount("/v1/responses", h.Routes())
	return r
}

func TestResponseHandler_Create_NonStreaming(t *testing.T) {
	h, s := newTestResponseHandler()
	router := newTestResponseRouter(h)

	body := `{
		"model": "gpt-4o",
		"input": "Hello, how are you?"
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp model.Response
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(resp.ID, "resp_"))
	assert.Equal(t, "response", resp.Object)
	assert.Equal(t, "completed", resp.Status)
	assert.Equal(t, "gpt-4o", resp.Model)
	assert.Positive(t, resp.CreatedAt)
	assert.Nil(t, resp.Error)

	// Output.
	require.Len(t, resp.Output, 1)
	assert.Equal(t, "message", resp.Output[0].Type)
	assert.True(t, strings.HasPrefix(resp.Output[0].ID, "msg_"))
	assert.Equal(t, "assistant", resp.Output[0].Role)
	assert.Equal(t, "completed", resp.Output[0].Status)
	require.Len(t, resp.Output[0].Content, 1)
	assert.Equal(t, "output_text", resp.Output[0].Content[0].Type)
	assert.NotEmpty(t, resp.Output[0].Content[0].Text)

	// Usage.
	assert.Positive(t, resp.Usage.InputTokens)
	assert.Positive(t, resp.Usage.OutputTokens)
	assert.Equal(t, resp.Usage.InputTokens+resp.Usage.OutputTokens, resp.Usage.TotalTokens)

	// Verify stored.
	stored, err := s.GetResponse(resp.ID)
	require.NoError(t, err)
	assert.Equal(t, resp.ID, stored.ID)
}

func TestResponseHandler_Create_WithMetadata(t *testing.T) {
	h, _ := newTestResponseHandler()
	router := newTestResponseRouter(h)

	body := `{
		"model": "gpt-4o",
		"input": "Hello",
		"metadata": {"env": "test", "trace_id": "abc123"}
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp model.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, "test", resp.Metadata["env"])
	assert.Equal(t, "abc123", resp.Metadata["trace_id"])
}

func TestResponseHandler_Create_WithInputItems(t *testing.T) {
	h, s := newTestResponseHandler()
	router := newTestResponseRouter(h)

	body := `{
		"model": "gpt-4o",
		"input": [
			{
				"type": "message",
				"role": "user",
				"content": [{"type": "input_text", "text": "Hello"}]
			}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp model.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	// Input items should be stored.
	items, err := s.ListResponseInputItems(resp.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "message", items[0].Type)
	assert.Equal(t, "user", items[0].Role)
}

func TestResponseHandler_Create_MissingModel(t *testing.T) {
	h, _ := newTestResponseHandler()
	router := newTestResponseRouter(h)

	body := `{"input": "Hello"}`

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
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

func TestResponseHandler_Create_MissingInput(t *testing.T) {
	h, _ := newTestResponseHandler()
	router := newTestResponseRouter(h)

	body := `{"model": "gpt-4o"}`

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp model.Error
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Contains(t, resp.Error.Message, "input")
	require.NotNil(t, resp.Error.Param)
	assert.Equal(t, "input", *resp.Error.Param)
}

func TestResponseHandler_Create_InvalidJSON(t *testing.T) {
	h, _ := newTestResponseHandler()
	router := newTestResponseRouter(h)

	body := `{not valid}`

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp model.Error
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, model.ErrTypeInvalidRequest, resp.Error.Type)
	assert.Contains(t, resp.Error.Message, "parse")
}

func TestResponseHandler_Create_EmptyBody(t *testing.T) {
	h, _ := newTestResponseHandler()
	router := newTestResponseRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", http.NoBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestResponseHandler_Create_UsesRequestModel(t *testing.T) {
	h, _ := newTestResponseHandler()
	router := newTestResponseRouter(h)

	body := `{
		"model": "o3-mini",
		"input": "Hello"
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp model.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, "o3-mini", resp.Model)
}

func TestResponseHandler_Create_Streaming(t *testing.T) {
	h, s := newTestResponseHandler()
	router := newTestResponseRouter(h)

	body := `{
		"model": "gpt-4o",
		"input": "Hello",
		"stream": true
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", w.Header().Get("Connection"))

	// Parse SSE events.
	events := parseResponseSSEEvents(t, w.Body.String())

	// Expected event sequence.
	expectedTypes := []string{
		model.EventResponseCreated,
		model.EventResponseInProgress,
		model.EventOutputItemAdded,
		model.EventContentPartAdded,
		model.EventOutputTextDelta,
		model.EventOutputTextDone,
		model.EventContentPartDone,
		model.EventOutputItemDone,
		model.EventResponseCompleted,
	}

	require.Len(t, events, len(expectedTypes), "expected %d events", len(expectedTypes))

	for i, expected := range expectedTypes {
		assert.Equal(t, expected, events[i].eventName, "event %d", i)
		assert.Equal(t, expected, events[i].data.Type, "event %d type field", i)
		assert.Equal(t, i, events[i].data.SequenceNumber, "event %d sequence", i)
	}

	// [DONE] sentinel.
	assert.Contains(t, w.Body.String(), "data: [DONE]")

	// Verify the response was stored by extracting the ID from the completed event.
	completedEvent := events[len(events)-1]
	var completedResp model.Response
	rawResp, marshalErr := json.Marshal(completedEvent.data.Response)
	require.NoError(t, marshalErr)
	require.NoError(t, json.Unmarshal(rawResp, &completedResp))

	storedResp, getErr := s.GetResponse(completedResp.ID)
	require.NoError(t, getErr)
	assert.Equal(t, completedResp.ID, storedResp.ID)
	assert.Equal(t, "completed", storedResp.Status)
}

func TestResponseHandler_Create_StreamFalse(t *testing.T) {
	h, _ := newTestResponseHandler()
	router := newTestResponseRouter(h)

	body := `{
		"model": "gpt-4o",
		"input": "Hello",
		"stream": false
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp model.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, "response", resp.Object)
	assert.Equal(t, "completed", resp.Status)
}

func TestResponseHandler_Create_UniqueIDs(t *testing.T) {
	h, _ := newTestResponseHandler()
	router := newTestResponseRouter(h)

	body := `{
		"model": "gpt-4o",
		"input": "Hello"
	}`

	ids := make(map[string]bool, 3)
	for range 3 {
		req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)

		var resp model.Response
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

		ids[resp.ID] = true
	}

	assert.Len(t, ids, 3, "all IDs should be unique")
}

// --- Get tests ---

func TestResponseHandler_Get_Success(t *testing.T) {
	h, _ := newTestResponseHandler()
	router := newTestResponseRouter(h)

	id := createResponse(t, router)

	req := httptest.NewRequest(http.MethodGet, "/v1/responses/"+id, http.NoBody)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp model.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, id, resp.ID)
	assert.Equal(t, "response", resp.Object)
	assert.Equal(t, "completed", resp.Status)
}

func TestResponseHandler_Get_NotFound(t *testing.T) {
	h, _ := newTestResponseHandler()
	router := newTestResponseRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/responses/nonexistent-id", http.NoBody)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp model.Error
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, model.ErrCodeNotFound, resp.Error.Code)
	assert.Contains(t, resp.Error.Message, "nonexistent-id")
}

// --- Delete tests ---

func TestResponseHandler_Delete_Success(t *testing.T) {
	h, _ := newTestResponseHandler()
	router := newTestResponseRouter(h)

	id := createResponse(t, router)

	req := httptest.NewRequest(http.MethodDelete, "/v1/responses/"+id, http.NoBody)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp model.DeleteResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, id, resp.ID)
	assert.Equal(t, "response.deleted", resp.Object)
	assert.True(t, resp.Deleted)

	// Verify it's actually gone.
	getReq := httptest.NewRequest(http.MethodGet, "/v1/responses/"+id, http.NoBody)
	getW := httptest.NewRecorder()
	router.ServeHTTP(getW, getReq)
	assert.Equal(t, http.StatusNotFound, getW.Code)
}

func TestResponseHandler_Delete_NotFound(t *testing.T) {
	h, _ := newTestResponseHandler()
	router := newTestResponseRouter(h)

	req := httptest.NewRequest(http.MethodDelete, "/v1/responses/nonexistent-id", http.NoBody)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- Cancel tests ---

func TestResponseHandler_Cancel_Success(t *testing.T) {
	h, _ := newTestResponseHandler()
	router := newTestResponseRouter(h)

	id := createResponse(t, router)

	req := httptest.NewRequest(http.MethodPost, "/v1/responses/"+id+"/cancel", http.NoBody)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp model.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, id, resp.ID)
	assert.Equal(t, "canceled", resp.Status)
}

func TestResponseHandler_Cancel_NotFound(t *testing.T) {
	h, _ := newTestResponseHandler()
	router := newTestResponseRouter(h)

	req := httptest.NewRequest(http.MethodPost, "/v1/responses/nonexistent-id/cancel", http.NoBody)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- ListInputItems tests ---

func TestResponseHandler_ListInputItems_Success(t *testing.T) {
	h, _ := newTestResponseHandler()
	router := newTestResponseRouter(h)

	// Create a response with input items.
	body := `{
		"model": "gpt-4o",
		"input": [
			{
				"type": "message",
				"id": "item_001",
				"role": "user",
				"content": [{"type": "input_text", "text": "Hello"}]
			},
			{
				"type": "message",
				"id": "item_002",
				"role": "user",
				"content": [{"type": "input_text", "text": "World"}]
			}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var created model.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))

	// List input items.
	listReq := httptest.NewRequest(http.MethodGet, "/v1/responses/"+created.ID+"/input_items", http.NoBody)
	listW := httptest.NewRecorder()
	router.ServeHTTP(listW, listReq)

	require.Equal(t, http.StatusOK, listW.Code)

	var resp model.ListResponse
	require.NoError(t, json.Unmarshal(listW.Body.Bytes(), &resp))

	assert.Equal(t, "list", resp.Object)
	assert.False(t, resp.HasMore)
	assert.Equal(t, "item_001", resp.FirstID)
	assert.Equal(t, "item_002", resp.LastID)

	var items []model.ResponseInputItem
	require.NoError(t, json.Unmarshal(resp.Data, &items))
	require.Len(t, items, 2)
	assert.Equal(t, "user", items[0].Role)
}

func TestResponseHandler_ListInputItems_Empty(t *testing.T) {
	h, _ := newTestResponseHandler()
	router := newTestResponseRouter(h)

	// Create a response with string input (no parseable items).
	id := createResponse(t, router)

	listReq := httptest.NewRequest(http.MethodGet, "/v1/responses/"+id+"/input_items", http.NoBody)
	listW := httptest.NewRecorder()
	router.ServeHTTP(listW, listReq)

	require.Equal(t, http.StatusOK, listW.Code)

	var resp model.ListResponse
	require.NoError(t, json.Unmarshal(listW.Body.Bytes(), &resp))

	assert.Equal(t, "list", resp.Object)

	var items []model.ResponseInputItem
	require.NoError(t, json.Unmarshal(resp.Data, &items))
	assert.Empty(t, items)
}

func TestResponseHandler_ListInputItems_NotFound(t *testing.T) {
	h, _ := newTestResponseHandler()
	router := newTestResponseRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/v1/responses/nonexistent-id/input_items", http.NoBody)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- CountTokens tests ---

func TestResponseHandler_CountTokens_Success(t *testing.T) {
	h, _ := newTestResponseHandler()
	router := newTestResponseRouter(h)

	body := `{
		"model": "gpt-4o",
		"input": "Hello, how are you?"
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/responses/count_tokens", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp model.CountTokensResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Positive(t, resp.TotalTokens)
}

func TestResponseHandler_CountTokens_MissingModel(t *testing.T) {
	h, _ := newTestResponseHandler()
	router := newTestResponseRouter(h)

	body := `{"input": "Hello"}`

	req := httptest.NewRequest(http.MethodPost, "/v1/responses/count_tokens", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp model.Error
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Contains(t, resp.Error.Message, "model")
}

func TestResponseHandler_CountTokens_MissingInput(t *testing.T) {
	h, _ := newTestResponseHandler()
	router := newTestResponseRouter(h)

	body := `{"model": "gpt-4o"}`

	req := httptest.NewRequest(http.MethodPost, "/v1/responses/count_tokens", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestResponseHandler_CountTokens_InvalidJSON(t *testing.T) {
	h, _ := newTestResponseHandler()
	router := newTestResponseRouter(h)

	body := `{not valid}`

	req := httptest.NewRequest(http.MethodPost, "/v1/responses/count_tokens", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- helpers ---

// createResponse creates a single response via the handler and returns its ID.
func createResponse(t *testing.T, router *chi.Mux) string {
	t.Helper()

	body := `{
		"model": "gpt-4o",
		"input": "Hello"
	}`

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var resp model.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	return resp.ID
}

// responseSSEEvent holds a parsed SSE event from the Responses API stream.
type responseSSEEvent struct {
	eventName string
	data      model.ResponseStreamEvent
}

// parseResponseSSEEvents extracts ResponseStreamEvent objects from an SSE stream,
// skipping the [DONE] sentinel.
func parseResponseSSEEvents(t *testing.T, body string) []responseSSEEvent {
	t.Helper()

	var events []responseSSEEvent
	scanner := bufio.NewScanner(strings.NewReader(body))

	var currentEvent string
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "event: ") {
			currentEvent = strings.TrimPrefix(line, "event: ")
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		payload := strings.TrimPrefix(line, "data: ")
		if payload == model.StreamDone {
			continue
		}

		var event model.ResponseStreamEvent
		err := json.Unmarshal([]byte(payload), &event)
		require.NoError(t, err, "failed to parse event: %s", payload)

		events = append(events, responseSSEEvent{
			eventName: currentEvent,
			data:      event,
		})
		currentEvent = ""
	}

	return events
}
