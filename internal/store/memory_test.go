package store_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asadk23/mock-ai/internal/model"
	"github.com/asadk23/mock-ai/internal/store"
)

func newTestCompletion(id, mdl string) model.ChatCompletion {
	return model.ChatCompletion{
		ID:      id,
		Object:  "chat.completion",
		Created: 1700000000,
		Model:   mdl,
		Choices: []model.ChatChoice{
			{
				Index: 0,
				Message: model.ChatMessage{
					Role:    "assistant",
					Content: "Hello!",
				},
				FinishReason: "stop",
			},
		},
		Usage: model.Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}
}

func newTestMessages() []model.ChatMessage {
	return []model.ChatMessage{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "Hi"},
	}
}

func newTestResponse(id, mdl string) model.Response {
	return model.Response{
		ID:        id,
		Object:    "response",
		CreatedAt: 1700000000,
		Status:    "completed",
		Model:     mdl,
		Output: []model.ResponseOutput{
			{
				Type: "message",
				ID:   "msg_001",
				Role: "assistant",
				Content: []model.ResponseContent{
					{Type: "output_text", Text: "Hello!"},
				},
			},
		},
		Usage: model.ResponseUsage{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
	}
}

func newTestInputItems() []model.ResponseInputItem {
	return []model.ResponseInputItem{
		{
			Type: "message",
			ID:   "item_001",
			Role: "user",
			Content: []model.ResponseContent{
				{Type: "input_text", Text: "Hi"},
			},
		},
	}
}

// Chat completion tests.

func TestMemory_CreateAndGetChatCompletion(t *testing.T) {
	s := store.NewMemory()
	comp := newTestCompletion("chatcmpl-1", "gpt-4o")
	msgs := newTestMessages()

	err := s.CreateChatCompletion(&comp, msgs)
	require.NoError(t, err)

	got, err := s.GetChatCompletion("chatcmpl-1")
	require.NoError(t, err)
	assert.Equal(t, "chatcmpl-1", got.ID)
	assert.Equal(t, "gpt-4o", got.Model)
	assert.Equal(t, "chat.completion", got.Object)
	assert.Len(t, got.Choices, 1)
}

func TestMemory_GetChatCompletion_NotFound(t *testing.T) {
	s := store.NewMemory()

	_, err := s.GetChatCompletion("nonexistent")
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestMemory_ListChatCompletions_Empty(t *testing.T) {
	s := store.NewMemory()

	result := s.ListChatCompletions()
	assert.Empty(t, result)
}

func TestMemory_ListChatCompletions_InsertionOrder(t *testing.T) {
	s := store.NewMemory()

	for _, id := range []string{"chatcmpl-a", "chatcmpl-b", "chatcmpl-c"} {
		c := newTestCompletion(id, "gpt-4o")
		err := s.CreateChatCompletion(&c, newTestMessages())
		require.NoError(t, err)
	}

	result := s.ListChatCompletions()
	require.Len(t, result, 3)
	assert.Equal(t, "chatcmpl-a", result[0].ID)
	assert.Equal(t, "chatcmpl-b", result[1].ID)
	assert.Equal(t, "chatcmpl-c", result[2].ID)
}

func TestMemory_UpdateChatCompletionMetadata(t *testing.T) {
	s := store.NewMemory()
	comp := newTestCompletion("chatcmpl-1", "gpt-4o")

	err := s.CreateChatCompletion(&comp, newTestMessages())
	require.NoError(t, err)

	meta := map[string]string{"key": "value", "env": "test"}
	updated, err := s.UpdateChatCompletionMetadata("chatcmpl-1", meta)
	require.NoError(t, err)
	assert.Equal(t, meta, updated.Metadata)

	// Verify mutation safety: changing the input map should not affect the store.
	meta["key"] = "changed"
	got, err := s.GetChatCompletion("chatcmpl-1")
	require.NoError(t, err)
	assert.Equal(t, "value", got.Metadata["key"])
}

func TestMemory_UpdateChatCompletionMetadata_NotFound(t *testing.T) {
	s := store.NewMemory()

	_, err := s.UpdateChatCompletionMetadata("nonexistent", map[string]string{"k": "v"})
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestMemory_DeleteChatCompletion(t *testing.T) {
	s := store.NewMemory()

	comp := newTestCompletion("chatcmpl-1", "gpt-4o")
	err := s.CreateChatCompletion(&comp, newTestMessages())
	require.NoError(t, err)

	err = s.DeleteChatCompletion("chatcmpl-1")
	require.NoError(t, err)

	_, err = s.GetChatCompletion("chatcmpl-1")
	require.ErrorIs(t, err, store.ErrNotFound)

	// List should also be empty.
	assert.Empty(t, s.ListChatCompletions())
}

func TestMemory_DeleteChatCompletion_NotFound(t *testing.T) {
	s := store.NewMemory()

	err := s.DeleteChatCompletion("nonexistent")
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestMemory_ListChatCompletionMessages(t *testing.T) {
	s := store.NewMemory()
	msgs := newTestMessages()

	comp := newTestCompletion("chatcmpl-1", "gpt-4o")
	err := s.CreateChatCompletion(&comp, msgs)
	require.NoError(t, err)

	got, err := s.ListChatCompletionMessages("chatcmpl-1")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "system", got[0].Role)
	assert.Equal(t, "user", got[1].Role)
}

func TestMemory_ListChatCompletionMessages_NotFound(t *testing.T) {
	s := store.NewMemory()

	_, err := s.ListChatCompletionMessages("nonexistent")
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestMemory_ListChatCompletionMessages_MutationSafety(t *testing.T) {
	s := store.NewMemory()

	comp := newTestCompletion("chatcmpl-1", "gpt-4o")
	err := s.CreateChatCompletion(&comp, newTestMessages())
	require.NoError(t, err)

	got, err := s.ListChatCompletionMessages("chatcmpl-1")
	require.NoError(t, err)

	// Mutate the returned slice.
	got[0].Role = "mutated"

	// Retrieve again and verify the store is unaffected.
	got2, err := s.ListChatCompletionMessages("chatcmpl-1")
	require.NoError(t, err)
	assert.Equal(t, "system", got2[0].Role)
}

// Response tests.

func TestMemory_CreateAndGetResponse(t *testing.T) {
	s := store.NewMemory()
	resp := newTestResponse("resp_001", "gpt-4o")
	items := newTestInputItems()

	err := s.CreateResponse(&resp, items)
	require.NoError(t, err)

	got, err := s.GetResponse("resp_001")
	require.NoError(t, err)
	assert.Equal(t, "resp_001", got.ID)
	assert.Equal(t, "response", got.Object)
	assert.Equal(t, "completed", got.Status)
	assert.Len(t, got.Output, 1)
}

func TestMemory_GetResponse_NotFound(t *testing.T) {
	s := store.NewMemory()

	_, err := s.GetResponse("nonexistent")
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestMemory_DeleteResponse(t *testing.T) {
	s := store.NewMemory()

	resp := newTestResponse("resp_001", "gpt-4o")
	err := s.CreateResponse(&resp, newTestInputItems())
	require.NoError(t, err)

	err = s.DeleteResponse("resp_001")
	require.NoError(t, err)

	_, err = s.GetResponse("resp_001")
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestMemory_DeleteResponse_NotFound(t *testing.T) {
	s := store.NewMemory()

	err := s.DeleteResponse("nonexistent")
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestMemory_CancelResponse(t *testing.T) {
	s := store.NewMemory()

	resp := newTestResponse("resp_001", "gpt-4o")
	resp.Status = "in_progress"

	err := s.CreateResponse(&resp, newTestInputItems())
	require.NoError(t, err)

	canceled, err := s.CancelResponse("resp_001")
	require.NoError(t, err)
	assert.Equal(t, "canceled", canceled.Status)

	// Verify the status persisted in the store.
	got, err := s.GetResponse("resp_001")
	require.NoError(t, err)
	assert.Equal(t, "canceled", got.Status)
}

func TestMemory_CancelResponse_NotFound(t *testing.T) {
	s := store.NewMemory()

	_, err := s.CancelResponse("nonexistent")
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestMemory_ListResponseInputItems(t *testing.T) {
	s := store.NewMemory()
	items := newTestInputItems()

	resp := newTestResponse("resp_001", "gpt-4o")
	err := s.CreateResponse(&resp, items)
	require.NoError(t, err)

	got, err := s.ListResponseInputItems("resp_001")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "message", got[0].Type)
	assert.Equal(t, "user", got[0].Role)
}

func TestMemory_ListResponseInputItems_NotFound(t *testing.T) {
	s := store.NewMemory()

	_, err := s.ListResponseInputItems("nonexistent")
	require.ErrorIs(t, err, store.ErrNotFound)
}

func TestMemory_ListResponseInputItems_MutationSafety(t *testing.T) {
	s := store.NewMemory()

	resp := newTestResponse("resp_001", "gpt-4o")
	err := s.CreateResponse(&resp, newTestInputItems())
	require.NoError(t, err)

	got, err := s.ListResponseInputItems("resp_001")
	require.NoError(t, err)

	// Mutate the returned slice.
	got[0].Role = "mutated"

	// Retrieve again and verify the store is unaffected.
	got2, err := s.ListResponseInputItems("resp_001")
	require.NoError(t, err)
	assert.Equal(t, "user", got2[0].Role)
}
