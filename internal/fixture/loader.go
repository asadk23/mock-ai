// Package fixture provides configurable mock response data for the mock-ai server.
//
// Default fixtures are compiled into the binary so the server works out of the
// box. Custom fixtures can be loaded from JSON files on disk, configured via
// the server's YAML config.
//
// Fixtures provide the static content template (e.g., response text, model name).
// Handlers overlay dynamic fields (IDs, timestamps, usage) at request time.
package fixture

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/asadk23/mock-ai/internal/config"
)

// Fixtures holds the loaded fixture data for all API groups.
type Fixtures struct {
	ChatCompletion     ChatCompletionFixture
	Response           ResponseFixture
	AudioTranscription AudioTranscriptionFixture
}

// ChatCompletionFixture defines the template data for chat completion responses.
type ChatCompletionFixture struct {
	// Content is the assistant's response text.
	Content string `json:"content"`

	// Model is the model name to include in responses.
	Model string `json:"model"`

	// SystemFingerprint is included in response objects.
	SystemFingerprint string `json:"system_fingerprint"`

	// PromptTokens is the default prompt token count.
	PromptTokens int `json:"prompt_tokens"`

	// CompletionTokens is the default completion token count.
	CompletionTokens int `json:"completion_tokens"`
}

// ResponseFixture defines the template data for Responses API responses.
type ResponseFixture struct {
	// Content is the assistant's response text.
	Content string `json:"content"`

	// Model is the model name to include in responses.
	Model string `json:"model"`

	// InputTokens is the default input token count.
	InputTokens int `json:"input_tokens"`

	// OutputTokens is the default output token count.
	OutputTokens int `json:"output_tokens"`
}

// AudioTranscriptionFixture defines the template data for audio transcription
// and translation responses.
type AudioTranscriptionFixture struct {
	// Text is the transcribed/translated text.
	Text string `json:"text"`
}

// Load reads fixture data from JSON files specified in the config. If a path
// is empty or the file does not exist, the compiled-in default is used.
func Load(cfg *config.FixtureConfig) (*Fixtures, error) {
	f := Default()

	if cfg.ChatCompletion != "" {
		if err := loadFromFile(cfg.ChatCompletion, &f.ChatCompletion); err != nil {
			return nil, fmt.Errorf("loading chat completion fixture: %w", err)
		}
	}

	if cfg.Responses != "" {
		if err := loadFromFile(cfg.Responses, &f.Response); err != nil {
			return nil, fmt.Errorf("loading responses fixture: %w", err)
		}
	}

	if cfg.AudioTranscription != "" {
		if err := loadFromFile(cfg.AudioTranscription, &f.AudioTranscription); err != nil {
			return nil, fmt.Errorf("loading audio transcription fixture: %w", err)
		}
	}

	return f, nil
}

// loadFromFile reads a JSON file and unmarshals it into the target.
func loadFromFile(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("parsing %s: %w", path, err)
	}

	return nil
}
