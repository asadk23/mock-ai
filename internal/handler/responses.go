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

// responseIDPrefix is the prefix for generated response IDs.
const responseIDPrefix = "resp_"

// outputItemIDPrefix is the prefix for generated output item IDs.
const outputItemIDPrefix = "msg_"

// ResponseHandler handles Responses API endpoints.
type ResponseHandler struct {
	store    store.Store
	fixtures *fixture.Fixtures
	logger   *zerolog.Logger
}

// NewResponseHandler creates a new ResponseHandler with the given dependencies.
func NewResponseHandler(s store.Store, f *fixture.Fixtures, logger *zerolog.Logger) *ResponseHandler {
	return &ResponseHandler{
		store:    s,
		fixtures: f,
		logger:   logger,
	}
}

// mockCountTokens is the default token count returned by the count_tokens endpoint.
const mockCountTokens = 25

// Routes returns a chi.Router with all response routes registered.
func (h *ResponseHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Post("/", h.Create)
	r.Post("/count_tokens", h.CountTokens)
	r.Get("/{response_id}", h.Get)
	r.Delete("/{response_id}", h.Delete)
	r.Post("/{response_id}/cancel", h.Cancel)
	r.Get("/{response_id}/input_items", h.ListInputItems)

	return r
}

// Create handles POST /v1/responses.
//
// It validates the request, generates a mock response using fixture data,
// stores it, and returns the response. If stream=true, the response is sent
// as Server-Sent Events with named event types.
//
//nolint:dupl // Structurally similar to ChatHandler.Create but operates on different types.
func (h *ResponseHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.ResponseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest,
			"We could not parse the JSON body of your request.",
			model.ErrTypeInvalidRequest, model.ErrCodeInvalidRequest)
		return
	}

	if err := validateResponseRequest(&req); err != nil {
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

// createNonStreaming builds and returns a complete Response JSON response.
func (h *ResponseHandler) createNonStreaming(w http.ResponseWriter, req *model.ResponseRequest) {
	f := h.fixtures.Response
	response := h.buildResponse(req, &f)

	inputItems := parseInputItems(req.Input)

	if err := h.store.CreateResponse(&response, inputItems); err != nil {
		h.logger.Error().Err(err).Msg("failed to store response")
		api.WriteError(w, http.StatusInternalServerError,
			"Internal server error.", model.ErrTypeServer, model.ErrCodeServerError)
		return
	}

	api.WriteJSON(w, http.StatusOK, response)
}

// createStreaming sends a Responses API SSE stream with named events.
//
// The stream format uses both "event:" and "data:" lines:
//  1. response.created — full response object (status: "in_progress")
//  2. response.in_progress — full response object
//  3. response.output_item.added — output message item
//  4. response.content_part.added — content part
//  5. response.output_text.delta — text content delta(s)
//  6. response.output_text.done — final text
//  7. response.content_part.done — completed content part
//  8. response.output_item.done — completed output item
//  9. response.completed — full response object (status: "completed")
//  10. data: [DONE]
func (h *ResponseHandler) createStreaming(w http.ResponseWriter, req *model.ResponseRequest) {
	sw, err := streaming.NewWriter(w)
	if err != nil {
		h.logger.Error().Err(err).Msg("streaming not supported")
		api.WriteError(w, http.StatusInternalServerError,
			"Streaming not supported.", model.ErrTypeServer, model.ErrCodeServerError)
		return
	}

	f := h.fixtures.Response
	response := h.buildResponse(req, &f)

	outputItem := response.Output[0]
	contentPart := outputItem.Content[0]
	seq := 0

	// 1. response.created (status: in_progress).
	inProgressResponse := response
	inProgressResponse.Status = "in_progress"
	inProgressResponse.Output = nil
	if err := h.writeStreamEvent(sw, model.EventResponseCreated, &seq, &model.ResponseStreamEvent{
		Response: inProgressResponse,
	}); err != nil {
		return
	}

	// 2. response.in_progress.
	if err := h.writeStreamEvent(sw, model.EventResponseInProgress, &seq, &model.ResponseStreamEvent{
		Response: inProgressResponse,
	}); err != nil {
		return
	}

	outputIndex := 0
	contentIndex := 0

	// 3. response.output_item.added.
	emptyItem := model.ResponseOutput{
		Type:    outputItem.Type,
		ID:      outputItem.ID,
		Status:  "in_progress",
		Role:    outputItem.Role,
		Content: []model.ResponseContent{},
	}
	if err := h.writeStreamEvent(sw, model.EventOutputItemAdded, &seq, &model.ResponseStreamEvent{
		Item:        emptyItem,
		OutputIndex: &outputIndex,
	}); err != nil {
		return
	}

	// 4. response.content_part.added.
	emptyPart := model.ResponseContent{
		Type:        contentPart.Type,
		Text:        "",
		Annotations: []any{},
	}
	if err := h.writeStreamEvent(sw, model.EventContentPartAdded, &seq, &model.ResponseStreamEvent{
		Part:         emptyPart,
		ItemID:       outputItem.ID,
		OutputIndex:  &outputIndex,
		ContentIndex: &contentIndex,
	}); err != nil {
		return
	}

	// 5. response.output_text.delta.
	if err := h.writeStreamEvent(sw, model.EventOutputTextDelta, &seq, &model.ResponseStreamEvent{
		Delta:        contentPart.Text,
		ItemID:       outputItem.ID,
		OutputIndex:  &outputIndex,
		ContentIndex: &contentIndex,
	}); err != nil {
		return
	}

	// 6. response.output_text.done.
	if err := h.writeStreamEvent(sw, model.EventOutputTextDone, &seq, &model.ResponseStreamEvent{
		Text:         contentPart.Text,
		ItemID:       outputItem.ID,
		OutputIndex:  &outputIndex,
		ContentIndex: &contentIndex,
	}); err != nil {
		return
	}

	// 7. response.content_part.done.
	if err := h.writeStreamEvent(sw, model.EventContentPartDone, &seq, &model.ResponseStreamEvent{
		Part:         contentPart,
		ItemID:       outputItem.ID,
		OutputIndex:  &outputIndex,
		ContentIndex: &contentIndex,
	}); err != nil {
		return
	}

	// 8. response.output_item.done.
	if err := h.writeStreamEvent(sw, model.EventOutputItemDone, &seq, &model.ResponseStreamEvent{
		Item:        outputItem,
		OutputIndex: &outputIndex,
	}); err != nil {
		return
	}

	// 9. response.completed.
	if err := h.writeStreamEvent(sw, model.EventResponseCompleted, &seq, &model.ResponseStreamEvent{
		Response: response,
	}); err != nil {
		return
	}

	// 10. Done sentinel.
	if err := sw.WriteDone(); err != nil {
		h.logger.Error().Err(err).Msg("failed to write done sentinel")
		return
	}

	// Store the response for later retrieval.
	inputItems := parseInputItems(req.Input)
	if err := h.store.CreateResponse(&response, inputItems); err != nil {
		h.logger.Error().Err(err).Msg("failed to store streamed response")
	}
}

// Get handles GET /v1/responses/{response_id}.
//
// It retrieves a single stored response by ID.
func (h *ResponseHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "response_id")

	response, err := h.store.GetResponse(id)
	if err != nil {
		api.WriteError(w, http.StatusNotFound,
			"No response found with id '"+id+"'.",
			model.ErrTypeNotFound, model.ErrCodeNotFound)
		return
	}

	api.WriteJSON(w, http.StatusOK, response)
}

// Delete handles DELETE /v1/responses/{response_id}.
//
// It removes a stored response and returns a deletion confirmation.
func (h *ResponseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "response_id")

	if err := h.store.DeleteResponse(id); err != nil {
		api.WriteError(w, http.StatusNotFound,
			"No response found with id '"+id+"'.",
			model.ErrTypeNotFound, model.ErrCodeNotFound)
		return
	}

	api.WriteJSON(w, http.StatusOK, model.DeleteResponse{
		ID:      id,
		Object:  "response.deleted",
		Deleted: true,
	})
}

// Cancel handles POST /v1/responses/{response_id}/cancel.
//
// It marks a response as canceled. In a real server this would stop
// in-progress generation; in the mock it simply updates the status.
func (h *ResponseHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "response_id")

	response, err := h.store.CancelResponse(id)
	if err != nil {
		api.WriteError(w, http.StatusNotFound,
			"No response found with id '"+id+"'.",
			model.ErrTypeNotFound, model.ErrCodeNotFound)
		return
	}

	api.WriteJSON(w, http.StatusOK, response)
}

// ListInputItems handles GET /v1/responses/{response_id}/input_items.
//
// It returns the input items that were sent with the original response request.
func (h *ResponseHandler) ListInputItems(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "response_id")

	items, err := h.store.ListResponseInputItems(id)
	if err != nil {
		api.WriteError(w, http.StatusNotFound,
			"No response found with id '"+id+"'.",
			model.ErrTypeNotFound, model.ErrCodeNotFound)
		return
	}

	if items == nil {
		items = []model.ResponseInputItem{}
	}

	data, marshalErr := json.Marshal(items)
	if marshalErr != nil {
		h.logger.Error().Err(marshalErr).Msg("failed to marshal input items list")
		api.WriteError(w, http.StatusInternalServerError,
			"Internal server error.", model.ErrTypeServer, model.ErrCodeServerError)
		return
	}

	resp := model.ListResponse{
		Object:  "list",
		Data:    data,
		HasMore: false,
	}

	if len(items) > 0 {
		resp.FirstID = items[0].ID
		resp.LastID = items[len(items)-1].ID
	}

	api.WriteJSON(w, http.StatusOK, resp)
}

// CountTokens handles POST /v1/responses/count_tokens.
//
// It returns a mock token count. In a real server this would tokenize the
// input; the mock returns a fixed count for testing purposes.
func (h *ResponseHandler) CountTokens(w http.ResponseWriter, r *http.Request) {
	var req model.CountTokensRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.WriteError(w, http.StatusBadRequest,
			"We could not parse the JSON body of your request.",
			model.ErrTypeInvalidRequest, model.ErrCodeInvalidRequest)
		return
	}

	if req.Model == "" {
		api.WriteErrorWithParam(w, http.StatusBadRequest,
			"you must provide a model parameter",
			model.ErrTypeInvalidRequest, model.ErrCodeMissingParam, "model")
		return
	}

	if len(req.Input) == 0 {
		api.WriteErrorWithParam(w, http.StatusBadRequest,
			"you must provide an input parameter",
			model.ErrTypeInvalidRequest, model.ErrCodeMissingParam, "input")
		return
	}

	api.WriteJSON(w, http.StatusOK, model.CountTokensResponse{
		TotalTokens: mockCountTokens,
	})
}

// writeStreamEvent writes a named SSE event with auto-incrementing sequence number.
func (h *ResponseHandler) writeStreamEvent(sw *streaming.Writer, eventType string, seq *int, event *model.ResponseStreamEvent) error {
	event.Type = eventType
	event.SequenceNumber = *seq
	*seq++

	if err := sw.WriteEvent(eventType, event); err != nil {
		h.logger.Error().Err(err).Str("event", eventType).Msg("failed to write stream event")
		return err
	}

	return nil
}

// buildResponse creates a Response object from the request and fixture data.
func (h *ResponseHandler) buildResponse(req *model.ResponseRequest, f *fixture.ResponseFixture) model.Response {
	responseModel := req.Model
	content := f.Content
	outputItemID := outputItemIDPrefix + uuid.New().String()[:24]

	return model.Response{
		ID:        responseIDPrefix + uuid.New().String()[:24],
		Object:    "response",
		CreatedAt: time.Now().Unix(),
		Status:    "completed",
		Model:     responseModel,
		Output: []model.ResponseOutput{
			{
				Type:   "message",
				ID:     outputItemID,
				Status: "completed",
				Role:   "assistant",
				Content: []model.ResponseContent{
					{
						Type:        "output_text",
						Text:        content,
						Annotations: []any{},
					},
				},
			},
		},
		Usage: model.ResponseUsage{
			InputTokens:  f.InputTokens,
			OutputTokens: f.OutputTokens,
			TotalTokens:  f.InputTokens + f.OutputTokens,
		},
		Metadata:     req.Metadata,
		Temperature:  req.Temperature,
		TopP:         req.TopP,
		MaxTokens:    req.MaxTokens,
		Instructions: req.Instructions,
	}
}

// parseInputItems attempts to parse the raw JSON input as an array of input items.
// If parsing fails (e.g., input is a plain string), it returns an empty slice.
func parseInputItems(input json.RawMessage) []model.ResponseInputItem {
	var items []model.ResponseInputItem
	if err := json.Unmarshal(input, &items); err != nil {
		return nil
	}
	return items
}

// validateResponseRequest checks required fields on the response request.
func validateResponseRequest(req *model.ResponseRequest) *validationError {
	if req.Model == "" {
		return &validationError{
			message: "you must provide a model parameter",
			param:   "model",
		}
	}

	if len(req.Input) == 0 {
		return &validationError{
			message: "you must provide an input parameter",
			param:   "input",
		}
	}

	return nil
}
