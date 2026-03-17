package streaming_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asadk23/mock-ai/internal/model"
	"github.com/asadk23/mock-ai/internal/streaming"
)

func TestNewWriter_SetsHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	sw, err := streaming.NewWriter(w)

	require.NoError(t, err)
	require.NotNil(t, sw)

	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", w.Header().Get("Connection"))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNewWriter_NonFlusher(t *testing.T) {
	w := &nonFlusherWriter{}
	sw, err := streaming.NewWriter(w)

	require.Error(t, err)
	assert.Nil(t, sw)
	assert.Contains(t, err.Error(), "http.Flusher")
}

func TestWriter_WriteData(t *testing.T) {
	w := httptest.NewRecorder()
	sw, err := streaming.NewWriter(w)
	require.NoError(t, err)

	chunk := model.ChatCompletionChunk{
		ID:      "chatcmpl-test123",
		Object:  model.ChatCompletionChunkObject,
		Created: 1694268190,
		Model:   "gpt-4o",
		Choices: []model.ChatChunkChoice{
			{
				Index: 0,
				Delta: model.ChatDelta{
					Role:    "assistant",
					Content: strPtr("Hello"),
				},
			},
		},
	}

	err = sw.WriteData(chunk)
	require.NoError(t, err)

	body := w.Body.String()
	lines := strings.Split(body, "\n")

	// Should have "data: {...}" followed by two newlines (empty line).
	require.GreaterOrEqual(t, len(lines), 2, "expected at least 2 lines")
	assert.True(t, strings.HasPrefix(lines[0], "data: "), "first line should start with 'data: '")

	// Should NOT have an "event:" line.
	assert.NotContains(t, body, "event:", "Chat Completions should not have event: lines")

	// Parse the JSON payload.
	jsonStr := strings.TrimPrefix(lines[0], "data: ")
	var parsed model.ChatCompletionChunk
	err = json.Unmarshal([]byte(jsonStr), &parsed)
	require.NoError(t, err)

	assert.Equal(t, "chatcmpl-test123", parsed.ID)
	assert.Equal(t, model.ChatCompletionChunkObject, parsed.Object)
	assert.Equal(t, "gpt-4o", parsed.Model)
	require.Len(t, parsed.Choices, 1)
	assert.Equal(t, "assistant", parsed.Choices[0].Delta.Role)
	assert.Equal(t, "Hello", *parsed.Choices[0].Delta.Content)
}

func TestWriter_WriteEvent(t *testing.T) {
	w := httptest.NewRecorder()
	sw, err := streaming.NewWriter(w)
	require.NoError(t, err)

	eventData := model.ResponseStreamEvent{
		Type:           model.EventResponseCreated,
		SequenceNumber: 0,
		Response: map[string]any{
			"id":     "resp_test123",
			"object": "response",
			"status": "in_progress",
		},
	}

	err = sw.WriteEvent(model.EventResponseCreated, eventData)
	require.NoError(t, err)

	body := w.Body.String()
	scanner := bufio.NewScanner(strings.NewReader(body))

	// First line should be "event: response.created".
	require.True(t, scanner.Scan())
	assert.Equal(t, "event: response.created", scanner.Text())

	// Second line should be "data: {...}".
	require.True(t, scanner.Scan())
	dataLine := scanner.Text()
	assert.True(t, strings.HasPrefix(dataLine, "data: "), "second line should start with 'data: '")

	// Parse the JSON payload.
	jsonStr := strings.TrimPrefix(dataLine, "data: ")
	var parsed model.ResponseStreamEvent
	err = json.Unmarshal([]byte(jsonStr), &parsed)
	require.NoError(t, err)

	assert.Equal(t, model.EventResponseCreated, parsed.Type)
	assert.Equal(t, 0, parsed.SequenceNumber)
	assert.NotNil(t, parsed.Response)
}

func TestWriter_WriteDone(t *testing.T) {
	w := httptest.NewRecorder()
	sw, err := streaming.NewWriter(w)
	require.NoError(t, err)

	err = sw.WriteDone()
	require.NoError(t, err)

	body := w.Body.String()
	assert.Contains(t, body, "data: [DONE]\n\n")
}

func TestWriter_FullChatStream(t *testing.T) {
	w := httptest.NewRecorder()
	sw, err := streaming.NewWriter(w)
	require.NoError(t, err)

	// Simulate a Chat Completions stream: role chunk, content chunk, finish chunk, done.
	chunks := []model.ChatCompletionChunk{
		{
			ID:      "chatcmpl-abc",
			Object:  model.ChatCompletionChunkObject,
			Created: 1694268190,
			Model:   "gpt-4o",
			Choices: []model.ChatChunkChoice{
				{Index: 0, Delta: model.ChatDelta{Role: "assistant", Content: strPtr("")}},
			},
		},
		{
			ID:      "chatcmpl-abc",
			Object:  model.ChatCompletionChunkObject,
			Created: 1694268190,
			Model:   "gpt-4o",
			Choices: []model.ChatChunkChoice{
				{Index: 0, Delta: model.ChatDelta{Content: strPtr("Hello")}},
			},
		},
		{
			ID:      "chatcmpl-abc",
			Object:  model.ChatCompletionChunkObject,
			Created: 1694268190,
			Model:   "gpt-4o",
			Choices: []model.ChatChunkChoice{
				{Index: 0, Delta: model.ChatDelta{}, FinishReason: strPtr("stop")},
			},
		},
	}

	for _, chunk := range chunks {
		err = sw.WriteData(chunk)
		require.NoError(t, err)
	}

	err = sw.WriteDone()
	require.NoError(t, err)

	// Verify the full output.
	body := w.Body.String()
	events := parseSSEDataEvents(t, body)

	require.Len(t, events, 3, "expected 3 data events before [DONE]")

	// All events should be bare data (no event: lines).
	assert.NotContains(t, body, "event:")

	// Last line should be the [DONE] sentinel.
	assert.Contains(t, body, "data: [DONE]\n\n")
}

func TestWriter_FullResponseStream(t *testing.T) {
	w := httptest.NewRecorder()
	sw, err := streaming.NewWriter(w)
	require.NoError(t, err)

	// Simulate a Responses API stream.
	events := []struct {
		name string
		data model.ResponseStreamEvent
	}{
		{
			name: model.EventResponseCreated,
			data: model.ResponseStreamEvent{
				Type:           model.EventResponseCreated,
				SequenceNumber: 0,
				Response:       map[string]string{"id": "resp_abc", "status": "in_progress"},
			},
		},
		{
			name: model.EventOutputTextDelta,
			data: model.ResponseStreamEvent{
				Type:           model.EventOutputTextDelta,
				SequenceNumber: 1,
				Delta:          "Hello",
				ItemID:         "msg_001",
				OutputIndex:    intPtr(0),
				ContentIndex:   intPtr(0),
			},
		},
		{
			name: model.EventResponseCompleted,
			data: model.ResponseStreamEvent{
				Type:           model.EventResponseCompleted,
				SequenceNumber: 2,
				Response:       map[string]string{"id": "resp_abc", "status": "completed"},
			},
		},
	}

	for _, e := range events {
		err = sw.WriteEvent(e.name, e.data)
		require.NoError(t, err)
	}

	err = sw.WriteDone()
	require.NoError(t, err)

	// Verify the output has named events.
	body := w.Body.String()

	assert.Contains(t, body, "event: response.created\n")
	assert.Contains(t, body, "event: response.output_text.delta\n")
	assert.Contains(t, body, "event: response.completed\n")
	assert.Contains(t, body, "data: [DONE]\n\n")
}

func TestWriter_WriteData_MarshalError(t *testing.T) {
	w := httptest.NewRecorder()
	sw, err := streaming.NewWriter(w)
	require.NoError(t, err)

	// Channels cannot be marshaled to JSON.
	err = sw.WriteData(make(chan int))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshal")
}

func TestWriter_WriteEvent_MarshalError(t *testing.T) {
	w := httptest.NewRecorder()
	sw, err := streaming.NewWriter(w)
	require.NoError(t, err)

	err = sw.WriteEvent("test.event", make(chan int))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "marshal")
}

func TestWriter_MultipleDataEvents_IndependentFlush(t *testing.T) {
	w := httptest.NewRecorder()
	sw, err := streaming.NewWriter(w)
	require.NoError(t, err)

	// Write two data events and verify both appear.
	err = sw.WriteData(map[string]string{"msg": "first"})
	require.NoError(t, err)

	err = sw.WriteData(map[string]string{"msg": "second"})
	require.NoError(t, err)

	body := w.Body.String()
	assert.Equal(t, 2, strings.Count(body, "data: "), "expected exactly 2 data lines")
	assert.Contains(t, body, `"msg":"first"`)
	assert.Contains(t, body, `"msg":"second"`)
}

// --- helpers ---

// errorWriter is an http.ResponseWriter that implements http.Flusher but
// always returns an error on Write. This is used to test the write-error path
// in WriteData, WriteEvent, and WriteDone.
type errorWriter struct {
	header http.Header
}

func newErrorWriter() *errorWriter {
	return &errorWriter{header: http.Header{}}
}

func (ew *errorWriter) Header() http.Header       { return ew.header }
func (ew *errorWriter) Write([]byte) (int, error) { return 0, errForcedWrite }
func (ew *errorWriter) WriteHeader(int)           {}
func (ew *errorWriter) Flush()                    {}

var errForcedWrite = fmt.Errorf("forced write error")

func TestWriter_WriteData_WriteError(t *testing.T) {
	ew := newErrorWriter()
	sw, err := streaming.NewWriter(ew)
	require.NoError(t, err)

	err = sw.WriteData(map[string]string{"msg": "hello"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write data")
}

func TestWriter_WriteEvent_WriteError(t *testing.T) {
	ew := newErrorWriter()
	sw, err := streaming.NewWriter(ew)
	require.NoError(t, err)

	err = sw.WriteEvent("test.event", map[string]string{"msg": "hello"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write event")
}

func TestWriter_WriteDone_WriteError(t *testing.T) {
	ew := newErrorWriter()
	sw, err := streaming.NewWriter(ew)
	require.NoError(t, err)

	err = sw.WriteDone()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "write done")
}

// nonFlusherWriter is an http.ResponseWriter that does NOT implement http.Flusher.
type nonFlusherWriter struct{}

func (nf *nonFlusherWriter) Header() http.Header       { return http.Header{} }
func (nf *nonFlusherWriter) Write([]byte) (int, error) { return 0, nil }
func (nf *nonFlusherWriter) WriteHeader(int)           {}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

// parseSSEDataEvents extracts bare "data:" payloads from SSE output, excluding [DONE].
func parseSSEDataEvents(t *testing.T, body string) []json.RawMessage {
	t.Helper()

	var events []json.RawMessage
	scanner := bufio.NewScanner(strings.NewReader(body))

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			payload := strings.TrimPrefix(line, "data: ")
			if payload == model.StreamDone {
				continue
			}
			events = append(events, json.RawMessage(payload))
		}
	}

	return events
}
