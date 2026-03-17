package fixture

// Default token counts for fixture responses.
const (
	_defaultPromptTokens     = 25
	_defaultCompletionTokens = 10
	_defaultInputTokens      = 25
	_defaultOutputTokens     = 10
)

// Default returns the compiled-in default fixtures. These provide sensible
// mock responses so the server works out of the box with zero configuration.
func Default() *Fixtures {
	return &Fixtures{
		ChatCompletion: ChatCompletionFixture{
			Content:           "Hello! How can I help you today?",
			Model:             "gpt-4o-2024-08-06",
			SystemFingerprint: "fp_mock_abc123",
			PromptTokens:      _defaultPromptTokens,
			CompletionTokens:  _defaultCompletionTokens,
		},
		Response: ResponseFixture{
			Content:      "Hello! How can I help you today?",
			Model:        "gpt-4o-2024-08-06",
			InputTokens:  _defaultInputTokens,
			OutputTokens: _defaultOutputTokens,
		},
		AudioTranscription: AudioTranscriptionFixture{
			Text: "Hello, this is a mock transcription of the audio file.",
		},
	}
}
