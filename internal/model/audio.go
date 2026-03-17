package model

// AudioSpeechRequest represents the request body for POST /v1/audio/speech.
type AudioSpeechRequest struct {
	Model          string   `json:"model"`
	Input          string   `json:"input"`
	Voice          string   `json:"voice"`
	ResponseFormat *string  `json:"response_format,omitempty"`
	Speed          *float64 `json:"speed,omitempty"`
}

// AudioTranscriptionResponse represents the response from POST /v1/audio/transcriptions.
type AudioTranscriptionResponse struct {
	Text string `json:"text"`
}

// AudioTranslationResponse represents the response from POST /v1/audio/translations.
type AudioTranslationResponse struct {
	Text string `json:"text"`
}

// _audioContentTypes maps supported audio response formats to their MIME types.
// This is the single source of truth for valid formats and their content types.
var _audioContentTypes = map[string]string{
	"mp3":  "audio/mpeg",
	"opus": "audio/opus",
	"aac":  "audio/aac",
	"flac": "audio/flac",
	"wav":  "audio/wav",
	"pcm":  "audio/pcm",
}

// IsValidAudioFormat reports whether the given format is a valid audio response format.
func IsValidAudioFormat(format string) bool {
	_, ok := _audioContentTypes[format]
	return ok
}

// AudioContentType returns the MIME content type for the given audio format.
// Returns an empty string if the format is not valid.
func AudioContentType(format string) string {
	return _audioContentTypes[format]
}
