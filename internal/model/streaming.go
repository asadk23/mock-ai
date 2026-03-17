package model

import "encoding/json"

// SSEEvent represents a server-sent event for streaming responses.
//
// Chat Completions uses bare "data:" lines only (Event is empty).
// Responses API uses named events: both "event:" and "data:" lines are sent.
//
// Both APIs terminate the stream with "data: [DONE]".
type SSEEvent struct {
	// Event is the SSE event type (e.g., "response.created", "response.completed").
	// For chat completions, this is empty (only "data:" lines are sent).
	Event string

	// Data is the JSON-encoded event payload.
	Data json.RawMessage
}

// StreamDone is the sentinel value indicating the end of any SSE stream.
// Both Chat Completions and Responses API use this same terminator.
const StreamDone = "[DONE]"

// Chat Completions streaming constants.
const (
	// ChatCompletionChunkObject is the object type for streaming chunks.
	ChatCompletionChunkObject = "chat.completion.chunk"
)

// Responses API SSE event types.
//
// These are the core event types emitted during a streaming response.
// The full OpenAI Responses API defines many more event types for built-in
// tools (web_search, file_search, code_interpreter, image_gen, MCP, etc.)
// which are out of scope for this mock server.
const (
	// Lifecycle events.
	EventResponseCreated    = "response.created"
	EventResponseInProgress = "response.in_progress"
	EventResponseCompleted  = "response.completed"
	EventResponseFailed     = "response.failed"
	EventResponseIncomplete = "response.incomplete"
	EventResponseCanceled   = "response.cancelled"
	EventResponseQueued     = "response.queued"

	// Output item events.
	EventOutputItemAdded = "response.output_item.added"
	EventOutputItemDone  = "response.output_item.done"

	// Content part events.
	EventContentPartAdded = "response.content_part.added"
	EventContentPartDone  = "response.content_part.done"

	// Text output events.
	EventOutputTextDelta         = "response.output_text.delta"
	EventOutputTextDone          = "response.output_text.done"
	EventOutputTextAnnotationAdd = "response.output_text.annotation.added"

	// Refusal events.
	EventRefusalDelta = "response.refusal.delta"
	EventRefusalDone  = "response.refusal.done"

	// Function call events.
	EventFunctionCallArgsDelta = "response.function_call_arguments.delta"
	EventFunctionCallArgsDone  = "response.function_call_arguments.done"

	// Error event.
	EventError = "error"
)

// ResponseStreamEvent represents a typed SSE event for the Responses API.
//
// Each event carries a "type" field matching the SSE event name, a
// monotonically increasing "sequence_number", and context fields
// (output_index, content_index, item_id) indicating position within
// the response structure.
type ResponseStreamEvent struct {
	Type           string `json:"type"`
	SequenceNumber int    `json:"sequence_number"`

	// Response is the full response object, present on lifecycle events
	// (response.created, response.in_progress, response.completed, etc.).
	Response any `json:"response,omitempty"`

	// Item is the output item, present on output_item.added/done events.
	Item any `json:"item,omitempty"`

	// Part is the content part, present on content_part.added/done events.
	Part any `json:"part,omitempty"`

	// Delta is the incremental text content for text/refusal delta events.
	Delta string `json:"delta,omitempty"`

	// Text is the complete text content for output_text.done/refusal.done events.
	Text string `json:"text,omitempty"`

	// ItemID identifies which output item this event belongs to.
	ItemID string `json:"item_id,omitempty"`

	// OutputIndex and ContentIndex track position in the response structure.
	OutputIndex  *int `json:"output_index,omitempty"`
	ContentIndex *int `json:"content_index,omitempty"`
}
