package model

// ChatCompletionRequest represents the request body for POST /v1/chat/completions.
type ChatCompletionRequest struct {
	Model            string              `json:"model"`
	Messages         []ChatMessage       `json:"messages"`
	Stream           *bool               `json:"stream,omitempty"`
	StreamOptions    *StreamOptions      `json:"stream_options,omitempty"`
	MaxTokens        *int                `json:"max_tokens,omitempty"`
	Temperature      *float64            `json:"temperature,omitempty"`
	TopP             *float64            `json:"top_p,omitempty"`
	N                *int                `json:"n,omitempty"`
	Stop             any                 `json:"stop,omitempty"`
	FrequencyPenalty *float64            `json:"frequency_penalty,omitempty"`
	PresencePenalty  *float64            `json:"presence_penalty,omitempty"`
	User             *string             `json:"user,omitempty"`
	Tools            []ChatTool          `json:"tools,omitempty"`
	ToolChoice       any                 `json:"tool_choice,omitempty"`
	ResponseFormat   *ChatResponseFormat `json:"response_format,omitempty"`
	Metadata         map[string]string   `json:"metadata,omitempty"`
}

// StreamOptions represents streaming configuration options.
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// ChatMessage represents a single message in the chat conversation.
type ChatMessage struct {
	Role       string     `json:"role"`
	Content    any        `json:"content"`
	Name       *string    `json:"name,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID *string    `json:"tool_call_id,omitempty"`
	Refusal    *string    `json:"refusal,omitempty"`
}

// ToolCall represents a tool call made by the model.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionCall `json:"function"`
}

// FunctionCall represents the function invocation in a tool call.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatTool represents a tool available to the model.
type ChatTool struct {
	Type     string       `json:"type"`
	Function ChatFunction `json:"function"`
}

// ChatFunction describes a function that can be called by the model.
type ChatFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

// ChatResponseFormat specifies the response format for chat completions.
type ChatResponseFormat struct {
	Type string `json:"type"`
}

// ChatCompletionUpdateRequest represents the request body for
// POST /v1/chat/completions/{completion_id} (update metadata).
type ChatCompletionUpdateRequest struct {
	Metadata map[string]string `json:"metadata"`
}

// ChatCompletion represents a chat completion response object.
type ChatCompletion struct {
	ID                string            `json:"id"`
	Object            string            `json:"object"`
	Created           int64             `json:"created"`
	Model             string            `json:"model"`
	Choices           []ChatChoice      `json:"choices"`
	Usage             Usage             `json:"usage"`
	ServiceTier       *string           `json:"service_tier,omitempty"`
	SystemFingerprint *string           `json:"system_fingerprint,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
}

// ChatChoice represents a single choice in a chat completion response.
type ChatChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
	Logprobs     *Logprobs   `json:"logprobs"`
}

// Logprobs represents log probability information for a choice.
type Logprobs struct {
	Content []TokenLogprob `json:"content,omitempty"`
}

// TokenLogprob represents log probability information for a single token.
type TokenLogprob struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
	Bytes   []int   `json:"bytes,omitempty"`
}

// ChatCompletionChunk represents a single chunk in a streaming chat completion.
type ChatCompletionChunk struct {
	ID                string            `json:"id"`
	Object            string            `json:"object"`
	Created           int64             `json:"created"`
	Model             string            `json:"model"`
	Choices           []ChatChunkChoice `json:"choices"`
	Usage             *Usage            `json:"usage,omitempty"`
	ServiceTier       *string           `json:"service_tier,omitempty"`
	SystemFingerprint *string           `json:"system_fingerprint,omitempty"`
}

// ChatChunkChoice represents a single choice in a streaming chunk.
type ChatChunkChoice struct {
	Index        int       `json:"index"`
	Delta        ChatDelta `json:"delta"`
	FinishReason *string   `json:"finish_reason"`
	Logprobs     *Logprobs `json:"logprobs"`
}

// ChatDelta represents the incremental content in a streaming chunk choice.
type ChatDelta struct {
	Role      string     `json:"role,omitempty"`
	Content   *string    `json:"content,omitempty"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Refusal   *string    `json:"refusal,omitempty"`
}
