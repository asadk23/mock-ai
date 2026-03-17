// Package streaming provides an SSE (Server-Sent Events) writer for streaming
// responses from the mock OpenAI API.
//
// It supports two wire formats:
//   - Chat Completions: bare "data:" lines only (no "event:" field).
//   - Responses API: named events with both "event:" and "data:" lines.
//
// Both formats terminate the stream with "data: [DONE]\n\n".
package streaming

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/asadk23/mock-ai/internal/model"
)

// streamingHeaders are the standard HTTP headers for SSE responses.
var _streamingHeaders = map[string]string{
	"Content-Type":  "text/event-stream",
	"Cache-Control": "no-cache",
	"Connection":    "keep-alive",
}

// Writer writes Server-Sent Events to an http.ResponseWriter.
//
// It handles JSON encoding, SSE framing, and flushing. The caller is
// responsible for ensuring the underlying ResponseWriter supports
// http.Flusher (all standard Go servers do).
type Writer struct {
	w       http.ResponseWriter
	flusher http.Flusher
}

// NewWriter creates a new SSE Writer and writes the streaming HTTP headers.
//
// It returns an error if the ResponseWriter does not implement http.Flusher.
// The caller should check this error and fall back to a non-streaming response
// if flushing is not supported.
func NewWriter(w http.ResponseWriter) (*Writer, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming: ResponseWriter does not implement http.Flusher")
	}

	for key, value := range _streamingHeaders {
		w.Header().Set(key, value)
	}
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	return &Writer{
		w:       w,
		flusher: flusher,
	}, nil
}

// WriteData writes a bare "data:" event (used by Chat Completions streaming).
//
// The data is JSON-encoded and written as:
//
//	data: {"id":"chatcmpl-abc",...}\n\n
func (sw *Writer) WriteData(data any) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("streaming: marshal data: %w", err)
	}

	if _, err := fmt.Fprintf(sw.w, "data: %s\n\n", jsonBytes); err != nil {
		return fmt.Errorf("streaming: write data: %w", err)
	}

	sw.flusher.Flush()
	return nil
}

// WriteEvent writes a named SSE event with both "event:" and "data:" lines
// (used by Responses API streaming).
//
// The output format is:
//
//	event: response.created\n
//	data: {"type":"response.created",...}\n\n
func (sw *Writer) WriteEvent(event string, data any) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("streaming: marshal event data: %w", err)
	}

	if _, err := fmt.Fprintf(sw.w, "event: %s\ndata: %s\n\n", event, jsonBytes); err != nil {
		return fmt.Errorf("streaming: write event: %w", err)
	}

	sw.flusher.Flush()
	return nil
}

// WriteDone writes the stream termination sentinel "data: [DONE]\n\n".
//
// Both Chat Completions and Responses API use this same terminator.
func (sw *Writer) WriteDone() error {
	if _, err := fmt.Fprintf(sw.w, "data: %s\n\n", model.StreamDone); err != nil {
		return fmt.Errorf("streaming: write done: %w", err)
	}

	sw.flusher.Flush()
	return nil
}
