package model

import "encoding/json"

// ResponseRequest represents the request body for POST /v1/responses.
type ResponseRequest struct {
	Model        string            `json:"model"`
	Input        json.RawMessage   `json:"input"`
	Stream       *bool             `json:"stream,omitempty"`
	Instructions *string           `json:"instructions,omitempty"`
	MaxTokens    *int              `json:"max_output_tokens,omitempty"`
	Temperature  *float64          `json:"temperature,omitempty"`
	TopP         *float64          `json:"top_p,omitempty"`
	Tools        []ResponseTool    `json:"tools,omitempty"`
	ToolChoice   any               `json:"tool_choice,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	User         *string           `json:"user,omitempty"`
}

// ResponseTool represents a tool available in the Responses API.
type ResponseTool struct {
	Type        string `json:"type"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

// Response represents a Responses API response object.
type Response struct {
	ID           string            `json:"id"`
	Object       string            `json:"object"`
	CreatedAt    int64             `json:"created_at"`
	Status       string            `json:"status"`
	Model        string            `json:"model"`
	Output       []ResponseOutput  `json:"output"`
	Usage        ResponseUsage     `json:"usage"`
	Error        *ResponseError    `json:"error"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Temperature  *float64          `json:"temperature,omitempty"`
	TopP         *float64          `json:"top_p,omitempty"`
	MaxTokens    *int              `json:"max_output_tokens,omitempty"`
	Instructions *string           `json:"instructions,omitempty"`
	User         *string           `json:"user,omitempty"`
}

// ResponseOutput represents an output item in a Responses API response.
type ResponseOutput struct {
	Type    string            `json:"type"`
	ID      string            `json:"id"`
	Status  string            `json:"status,omitempty"`
	Role    string            `json:"role,omitempty"`
	Content []ResponseContent `json:"content,omitempty"`
}

// ResponseContent represents a content part within a response output item.
type ResponseContent struct {
	Type        string `json:"type"`
	Text        string `json:"text,omitempty"`
	Annotations []any  `json:"annotations,omitempty"`
}

// ResponseUsage represents token usage in the Responses API format.
type ResponseUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// ResponseError represents an error within a response object.
type ResponseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ResponseInputItem represents an item in the input_items list.
type ResponseInputItem struct {
	Type    string            `json:"type"`
	ID      string            `json:"id"`
	Role    string            `json:"role,omitempty"`
	Content []ResponseContent `json:"content,omitempty"`
}

// CountTokensRequest represents the request body for POST /v1/responses/count_tokens.
type CountTokensRequest struct {
	Model string          `json:"model"`
	Input json.RawMessage `json:"input"`
}

// CountTokensResponse represents the response from the count tokens endpoint.
type CountTokensResponse struct {
	TotalTokens int `json:"total_tokens"`
}
