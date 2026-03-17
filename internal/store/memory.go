// Package store provides in-memory state management for the mock-ai server.
// All state is lost on server restart.
package store

import (
	"errors"
	"sync"

	"github.com/asadk23/mock-ai/internal/model"
)

// Common errors returned by store operations.
var (
	ErrNotFound = errors.New("not found")
)

// Store defines the interface for state management operations.
type Store interface {
	// Chat completion operations.
	CreateChatCompletion(completion *model.ChatCompletion, messages []model.ChatMessage) error
	GetChatCompletion(id string) (model.ChatCompletion, error)
	ListChatCompletions() []model.ChatCompletion
	UpdateChatCompletionMetadata(id string, metadata map[string]string) (model.ChatCompletion, error)
	DeleteChatCompletion(id string) error
	ListChatCompletionMessages(id string) ([]model.ChatMessage, error)

	// Response operations.
	CreateResponse(response *model.Response, inputItems []model.ResponseInputItem) error
	GetResponse(id string) (model.Response, error)
	DeleteResponse(id string) error
	CancelResponse(id string) (model.Response, error)
	ListResponseInputItems(id string) ([]model.ResponseInputItem, error)
}

// responseEntry holds a response and its associated input items.
type responseEntry struct {
	response   model.Response
	inputItems []model.ResponseInputItem
}

// completionEntry holds a chat completion and its associated messages.
type completionEntry struct {
	completion model.ChatCompletion
	messages   []model.ChatMessage
}

// Memory is a thread-safe in-memory implementation of Store.
type Memory struct {
	mu          sync.RWMutex
	completions map[string]completionEntry
	responses   map[string]responseEntry

	// ordered tracks insertion order for list operations.
	completionOrder []string
}

// Compile-time interface compliance check.
var _ Store = (*Memory)(nil)

// NewMemory creates a new empty in-memory store.
func NewMemory() *Memory {
	return &Memory{
		completions: make(map[string]completionEntry),
		responses:   make(map[string]responseEntry),
	}
}

// CreateChatCompletion stores a chat completion along with its input messages.
func (m *Memory) CreateChatCompletion(completion *model.ChatCompletion, messages []model.ChatMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	msgs := make([]model.ChatMessage, len(messages))
	copy(msgs, messages)

	m.completions[completion.ID] = completionEntry{
		completion: *completion,
		messages:   msgs,
	}
	m.completionOrder = append(m.completionOrder, completion.ID)

	return nil
}

// GetChatCompletion retrieves a chat completion by ID.
func (m *Memory) GetChatCompletion(id string) (model.ChatCompletion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.completions[id]
	if !ok {
		return model.ChatCompletion{}, ErrNotFound
	}

	return entry.completion, nil
}

// ListChatCompletions returns all stored chat completions in insertion order.
func (m *Memory) ListChatCompletions() []model.ChatCompletion {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]model.ChatCompletion, 0, len(m.completionOrder))
	for _, id := range m.completionOrder {
		if entry, ok := m.completions[id]; ok {
			result = append(result, entry.completion)
		}
	}

	return result
}

// UpdateChatCompletionMetadata replaces the metadata on a chat completion.
func (m *Memory) UpdateChatCompletionMetadata(id string, metadata map[string]string) (model.ChatCompletion, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.completions[id]
	if !ok {
		return model.ChatCompletion{}, ErrNotFound
	}

	// Copy metadata to avoid caller mutation.
	meta := make(map[string]string, len(metadata))
	for k, v := range metadata {
		meta[k] = v
	}

	entry.completion.Metadata = meta
	m.completions[id] = entry

	return entry.completion, nil
}

// DeleteChatCompletion removes a chat completion by ID.
func (m *Memory) DeleteChatCompletion(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.completions[id]; !ok {
		return ErrNotFound
	}

	delete(m.completions, id)

	// Remove from ordered list.
	for i, cid := range m.completionOrder {
		if cid == id {
			m.completionOrder = append(m.completionOrder[:i], m.completionOrder[i+1:]...)
			break
		}
	}

	return nil
}

// ListChatCompletionMessages returns the input messages for a chat completion.
func (m *Memory) ListChatCompletionMessages(id string) ([]model.ChatMessage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.completions[id]
	if !ok {
		return nil, ErrNotFound
	}

	// Return a copy to prevent caller mutation.
	msgs := make([]model.ChatMessage, len(entry.messages))
	copy(msgs, entry.messages)

	return msgs, nil
}

// CreateResponse stores a response along with its input items.
func (m *Memory) CreateResponse(response *model.Response, inputItems []model.ResponseInputItem) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	items := make([]model.ResponseInputItem, len(inputItems))
	copy(items, inputItems)

	m.responses[response.ID] = responseEntry{
		response:   *response,
		inputItems: items,
	}

	return nil
}

// GetResponse retrieves a response by ID.
func (m *Memory) GetResponse(id string) (model.Response, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.responses[id]
	if !ok {
		return model.Response{}, ErrNotFound
	}

	return entry.response, nil
}

// DeleteResponse removes a response by ID.
func (m *Memory) DeleteResponse(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.responses[id]; !ok {
		return ErrNotFound
	}

	delete(m.responses, id)

	return nil
}

// CancelResponse marks a response as canceled if it is currently in_progress.
// Returns the updated response or an error if not found.
func (m *Memory) CancelResponse(id string) (model.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.responses[id]
	if !ok {
		return model.Response{}, ErrNotFound
	}

	entry.response.Status = "canceled"
	m.responses[id] = entry

	return entry.response, nil
}

// ListResponseInputItems returns the input items for a response.
func (m *Memory) ListResponseInputItems(id string) ([]model.ResponseInputItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.responses[id]
	if !ok {
		return nil, ErrNotFound
	}

	// Return a copy to prevent caller mutation.
	items := make([]model.ResponseInputItem, len(entry.inputItems))
	copy(items, entry.inputItems)

	return items, nil
}
