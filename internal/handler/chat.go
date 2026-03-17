// Package handler implements HTTP route handlers for the mock OpenAI API.
package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"github.com/asadk23/mock-ai/internal/api"
	"github.com/asadk23/mock-ai/internal/fixture"
	"github.com/asadk23/mock-ai/internal/model"
	"github.com/asadk23/mock-ai/internal/store"
	"github.com/asadk23/mock-ai/internal/streaming"
)

// chatCompletionIDPrefix is the prefix for generated chat completion IDs.
const chatCompletionIDPrefix = "chatcmpl-"

// ChatHandler handles Chat Completions API endpoints.
type ChatHandler struct {
	store    store.Store
	fixtures *fixture.Fixtures
	logger   *zerolog.Logger
}

// NewChatHandler creates a new ChatHandler with the given dependencies.
func NewChatHandler(s store.Store, f *fixture.Fixtures, logger *zerolog.Logger) *ChatHandler {
	return &ChatHandler{
		store:    s,
		fixtures: f,
		logger:   logger,
	}
}

// Routes returns a chi.Router with all chat completion routes registered.
func (h *ChatHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Post("/", h.Create)
	r.Get("/", h.List)
	r.Get("/{completion_id}", h.Get)
	r.Post("/{completion_id}", h.Update)
	r.Delete("/{completion_id}", h.Delete)
	r.Get("/{completion_id}/messages", h.ListMessages)

	return r
}

// Create handles POST /v1/chat/completions.
//
// It validates the request, generates a mock completion using fixture data,
// stores it, and returns the response. If stream=true, the response is sent
// as Server-Sent Events.
//
//nolint:dupl // Structurally similar to ResponseHandler.Create but operates on different types.
func (h *ChatHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.ChatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest,
			"We could not parse the JSON body of your request.",
			model.ErrTypeInvalidRequest, model.ErrCodeInvalidRequest)
		return
	}

	if err := validateChatRequest(&req); err != nil {
		api.WriteErrorWithParam(w, http.StatusBadRequest,
			err.Error(), model.ErrTypeInvalidRequest, model.ErrCodeMissingParam, err.param)
		return
	}

	if req.Stream != nil && *req.Stream {
		h.createStreaming(w, &req)
		return
	}

	h.createNonStreaming(w, &req)
}

// createNonStreaming builds and returns a complete ChatCompletion JSON response.
func (h *ChatHandler) createNonStreaming(w http.ResponseWriter, req *model.ChatCompletionRequest) {
	f := h.fixtures.ChatCompletion

	completionModel := req.Model
	content := f.Content
	fingerprint := f.SystemFingerprint

	completion := model.ChatCompletion{
		ID:      chatCompletionIDPrefix + uuid.New().String()[:24],
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   completionModel,
		Choices: []model.ChatChoice{
			{
				Index: 0,
				Message: model.ChatMessage{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: "stop",
				Logprobs:     nil,
			},
		},
		Usage: model.Usage{
			PromptTokens:     f.PromptTokens,
			CompletionTokens: f.CompletionTokens,
			TotalTokens:      f.PromptTokens + f.CompletionTokens,
		},
		SystemFingerprint: &fingerprint,
		Metadata:          req.Metadata,
	}

	if err := h.store.CreateChatCompletion(&completion, req.Messages); err != nil {
		h.logger.Error().Err(err).Msg("failed to store chat completion")
		api.WriteError(w, http.StatusInternalServerError,
			"Internal server error.", model.ErrTypeServer, model.ErrCodeServerError)
		return
	}

	api.WriteJSON(w, http.StatusOK, completion)
}

// createStreaming sends a Chat Completions SSE stream.
//
// The stream format uses bare "data:" lines (no "event:" field):
//  1. Role chunk: delta with role="assistant" and empty content
//  2. Content chunks: one per "word" of fixture content
//  3. Finish chunk: delta with empty fields and finish_reason="stop"
//  4. Usage chunk (optional): if stream_options.include_usage is true
//  5. data: [DONE]
func (h *ChatHandler) createStreaming(w http.ResponseWriter, req *model.ChatCompletionRequest) {
	sw, err := streaming.NewWriter(w)
	if err != nil {
		h.logger.Error().Err(err).Msg("streaming not supported")
		api.WriteError(w, http.StatusInternalServerError,
			"Streaming not supported.", model.ErrTypeServer, model.ErrCodeServerError)
		return
	}

	f := h.fixtures.ChatCompletion

	completionID := chatCompletionIDPrefix + uuid.New().String()[:24]
	created := time.Now().Unix()
	completionModel := req.Model
	fingerprint := f.SystemFingerprint

	// 1. Role chunk.
	emptyContent := ""
	if err := sw.WriteData(model.ChatCompletionChunk{
		ID:      completionID,
		Object:  model.ChatCompletionChunkObject,
		Created: created,
		Model:   completionModel,
		Choices: []model.ChatChunkChoice{
			{
				Index:        0,
				Delta:        model.ChatDelta{Role: "assistant", Content: &emptyContent},
				FinishReason: nil,
			},
		},
		SystemFingerprint: &fingerprint,
	}); err != nil {
		h.logger.Error().Err(err).Msg("failed to write role chunk")
		return
	}

	// 2. Content chunk (single chunk with full content for mock simplicity).
	content := f.Content
	if err := sw.WriteData(model.ChatCompletionChunk{
		ID:      completionID,
		Object:  model.ChatCompletionChunkObject,
		Created: created,
		Model:   completionModel,
		Choices: []model.ChatChunkChoice{
			{
				Index:        0,
				Delta:        model.ChatDelta{Content: &content},
				FinishReason: nil,
			},
		},
		SystemFingerprint: &fingerprint,
	}); err != nil {
		h.logger.Error().Err(err).Msg("failed to write content chunk")
		return
	}

	// 3. Finish chunk.
	stopReason := "stop"
	if err := sw.WriteData(model.ChatCompletionChunk{
		ID:      completionID,
		Object:  model.ChatCompletionChunkObject,
		Created: created,
		Model:   completionModel,
		Choices: []model.ChatChunkChoice{
			{
				Index:        0,
				Delta:        model.ChatDelta{},
				FinishReason: &stopReason,
			},
		},
		SystemFingerprint: &fingerprint,
	}); err != nil {
		h.logger.Error().Err(err).Msg("failed to write finish chunk")
		return
	}

	// 4. Usage chunk (optional).
	includeUsage := req.StreamOptions != nil && req.StreamOptions.IncludeUsage
	if includeUsage {
		totalTokens := f.PromptTokens + f.CompletionTokens
		usage := model.Usage{
			PromptTokens:     f.PromptTokens,
			CompletionTokens: f.CompletionTokens,
			TotalTokens:      totalTokens,
		}
		if err := sw.WriteData(model.ChatCompletionChunk{
			ID:                completionID,
			Object:            model.ChatCompletionChunkObject,
			Created:           created,
			Model:             completionModel,
			Choices:           []model.ChatChunkChoice{},
			Usage:             &usage,
			SystemFingerprint: &fingerprint,
		}); err != nil {
			h.logger.Error().Err(err).Msg("failed to write usage chunk")
			return
		}
	}

	// 5. Done sentinel.
	if err := sw.WriteDone(); err != nil {
		h.logger.Error().Err(err).Msg("failed to write done sentinel")
		return
	}

	// Store the completion for later retrieval (GET/list).
	completion := model.ChatCompletion{
		ID:      completionID,
		Object:  "chat.completion",
		Created: created,
		Model:   completionModel,
		Choices: []model.ChatChoice{
			{
				Index: 0,
				Message: model.ChatMessage{
					Role:    "assistant",
					Content: content,
				},
				FinishReason: "stop",
				Logprobs:     nil,
			},
		},
		Usage: model.Usage{
			PromptTokens:     f.PromptTokens,
			CompletionTokens: f.CompletionTokens,
			TotalTokens:      f.PromptTokens + f.CompletionTokens,
		},
		SystemFingerprint: &fingerprint,
		Metadata:          req.Metadata,
	}

	if err := h.store.CreateChatCompletion(&completion, req.Messages); err != nil {
		h.logger.Error().Err(err).Msg("failed to store streamed chat completion")
	}
}

// List handles GET /v1/chat/completions.
//
// It returns all stored chat completions in insertion order wrapped in a
// list response object.
func (h *ChatHandler) List(w http.ResponseWriter, _ *http.Request) {
	completions := h.store.ListChatCompletions()

	data, err := json.Marshal(completions)
	if err != nil {
		h.logger.Error().Err(err).Msg("failed to marshal completions list")
		api.WriteError(w, http.StatusInternalServerError,
			"Internal server error.", model.ErrTypeServer, model.ErrCodeServerError)
		return
	}

	resp := model.ListResponse{
		Object:  "list",
		Data:    data,
		HasMore: false,
	}

	if len(completions) > 0 {
		resp.FirstID = completions[0].ID
		resp.LastID = completions[len(completions)-1].ID
	}

	api.WriteJSON(w, http.StatusOK, resp)
}

// Get handles GET /v1/chat/completions/{completion_id}.
//
// It retrieves a single stored chat completion by ID.
func (h *ChatHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "completion_id")

	completion, err := h.store.GetChatCompletion(id)
	if err != nil {
		api.WriteError(w, http.StatusNotFound,
			"No chat completion found with id '"+id+"'.",
			model.ErrTypeNotFound, model.ErrCodeNotFound)
		return
	}

	api.WriteJSON(w, http.StatusOK, completion)
}

// Update handles POST /v1/chat/completions/{completion_id}.
//
// It updates the metadata on an existing chat completion. OpenAI uses POST
// (not PUT/PATCH) for modification endpoints.
func (h *ChatHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "completion_id")

	var req model.ChatCompletionUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest,
			"We could not parse the JSON body of your request.",
			model.ErrTypeInvalidRequest, model.ErrCodeInvalidRequest)
		return
	}

	completion, err := h.store.UpdateChatCompletionMetadata(id, req.Metadata)
	if err != nil {
		api.WriteError(w, http.StatusNotFound,
			"No chat completion found with id '"+id+"'.",
			model.ErrTypeNotFound, model.ErrCodeNotFound)
		return
	}

	api.WriteJSON(w, http.StatusOK, completion)
}

// Delete handles DELETE /v1/chat/completions/{completion_id}.
//
// It removes a stored chat completion and returns a deletion confirmation.
func (h *ChatHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "completion_id")

	if err := h.store.DeleteChatCompletion(id); err != nil {
		api.WriteError(w, http.StatusNotFound,
			"No chat completion found with id '"+id+"'.",
			model.ErrTypeNotFound, model.ErrCodeNotFound)
		return
	}

	api.WriteJSON(w, http.StatusOK, model.DeleteResponse{
		ID:      id,
		Object:  "chat.completion.deleted",
		Deleted: true,
	})
}

// ListMessages handles GET /v1/chat/completions/{completion_id}/messages.
//
// It returns the input messages that were sent with the original chat completion
// request, wrapped in a list response object.
func (h *ChatHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "completion_id")

	messages, err := h.store.ListChatCompletionMessages(id)
	if err != nil {
		api.WriteError(w, http.StatusNotFound,
			"No chat completion found with id '"+id+"'.",
			model.ErrTypeNotFound, model.ErrCodeNotFound)
		return
	}

	data, marshalErr := json.Marshal(messages)
	if marshalErr != nil {
		h.logger.Error().Err(marshalErr).Msg("failed to marshal messages list")
		api.WriteError(w, http.StatusInternalServerError,
			"Internal server error.", model.ErrTypeServer, model.ErrCodeServerError)
		return
	}

	resp := model.ListResponse{
		Object:  "list",
		Data:    data,
		HasMore: false,
	}

	api.WriteJSON(w, http.StatusOK, resp)
}

// validateChatRequest checks required fields on the chat completion request.
func validateChatRequest(req *model.ChatCompletionRequest) *validationError {
	if req.Model == "" {
		return &validationError{
			message: "you must provide a model parameter",
			param:   "model",
		}
	}

	if len(req.Messages) == 0 {
		return &validationError{
			message: "[] is too short - 'messages'",
			param:   "messages",
		}
	}

	return nil
}
