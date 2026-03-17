// Package api provides HTTP response helpers for the mock-ai server.
package api

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/asadk23/mock-ai/internal/model"
)

// WriteJSON encodes data as JSON and writes it to the response writer with the
// given status code. The Content-Type header is set to application/json.
// JSON encoding is buffered so that encoding failures are returned as a 500
// error without corrupting a partially-written response.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(data); err != nil {
		http.Error(w, `{"error":{"message":"internal encoding error","type":"server_error","code":"server_error"}}`,
			http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// WriteError writes an OpenAI-compatible error response.
func WriteError(w http.ResponseWriter, status int, message, errType, code string) {
	WriteJSON(w, status, model.NewError(message, errType, code))
}

// WriteErrorWithParam writes an OpenAI-compatible error response that includes
// a parameter name.
func WriteErrorWithParam(w http.ResponseWriter, status int, message, errType, code, param string) {
	WriteJSON(w, status, model.NewErrorWithParam(message, errType, code, param))
}
