// Package model defines request and response types for the mock OpenAI API.
// All JSON tags use snake_case to match the OpenAI API format.
package model

import "encoding/json"

// Error represents an OpenAI-compatible error response.
type Error struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody contains the details of an API error.
type ErrorBody struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Param   *string `json:"param"`
	Code    string  `json:"code"`
}

// Common OpenAI error types.
// Note: OpenAI uses "invalid_request_error" for most client-side errors
// including auth failures and not-found responses. The distinct constants
// exist for semantic clarity at call sites, not for distinct string values.
const (
	ErrTypeInvalidRequest = "invalid_request_error"
	ErrTypeAuth           = "invalid_request_error"
	ErrTypeNotFound       = "invalid_request_error"
	ErrTypeServer         = "server_error"
)

// Common OpenAI error codes.
const (
	ErrCodeInvalidAPIKey  = "invalid_api_key"
	ErrCodeInvalidValue   = "invalid_value"
	ErrCodeMissingParam   = "missing_required_parameter"
	ErrCodeNotFound       = "not_found"
	ErrCodeServerError    = "server_error"
	ErrCodeInvalidRequest = "invalid_request"
)

// NewError creates an Error with the given details.
func NewError(message, errType, code string) Error {
	return Error{
		Error: ErrorBody{
			Message: message,
			Type:    errType,
			Param:   nil,
			Code:    code,
		},
	}
}

// NewErrorWithParam creates an Error that includes a parameter name.
func NewErrorWithParam(message, errType, code, param string) Error {
	return Error{
		Error: ErrorBody{
			Message: message,
			Type:    errType,
			Param:   &param,
			Code:    code,
		},
	}
}

// Usage represents token usage statistics returned by the API.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ListResponse is a generic wrapper for list endpoints that return paginated data.
type ListResponse struct {
	Object  string          `json:"object"`
	Data    json.RawMessage `json:"data"`
	FirstID string          `json:"first_id,omitempty"`
	LastID  string          `json:"last_id,omitempty"`
	HasMore bool            `json:"has_more"`
}

// DeleteResponse represents the response from a delete endpoint.
type DeleteResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Deleted bool   `json:"deleted"`
}
