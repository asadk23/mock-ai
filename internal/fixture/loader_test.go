package fixture_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asadk23/mock-ai/internal/config"
	"github.com/asadk23/mock-ai/internal/fixture"
)

func TestDefault_ReturnsNonEmptyFixtures(t *testing.T) {
	f := fixture.Default()

	assert.NotEmpty(t, f.ChatCompletion.Content)
	assert.NotEmpty(t, f.ChatCompletion.Model)
	assert.NotEmpty(t, f.ChatCompletion.SystemFingerprint)
	assert.Positive(t, f.ChatCompletion.PromptTokens)
	assert.Positive(t, f.ChatCompletion.CompletionTokens)

	assert.NotEmpty(t, f.Response.Content)
	assert.NotEmpty(t, f.Response.Model)
	assert.Positive(t, f.Response.InputTokens)
	assert.Positive(t, f.Response.OutputTokens)

	assert.NotEmpty(t, f.AudioTranscription.Text)
}

func TestLoad_EmptyConfig_ReturnsDefaults(t *testing.T) {
	cfg := &config.FixtureConfig{}
	f, err := fixture.Load(cfg)

	require.NoError(t, err)

	defaults := fixture.Default()
	assert.Equal(t, defaults.ChatCompletion, f.ChatCompletion)
	assert.Equal(t, defaults.Response, f.Response)
	assert.Equal(t, defaults.AudioTranscription, f.AudioTranscription)
}

func TestLoad_ChatCompletionFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chat.json")

	data := `{
		"content": "Custom chat response",
		"model": "gpt-4-turbo",
		"system_fingerprint": "fp_custom",
		"prompt_tokens": 100,
		"completion_tokens": 50
	}`

	err := os.WriteFile(path, []byte(data), 0o644)
	require.NoError(t, err)

	cfg := &config.FixtureConfig{ChatCompletion: path}
	f, err := fixture.Load(cfg)

	require.NoError(t, err)
	assert.Equal(t, "Custom chat response", f.ChatCompletion.Content)
	assert.Equal(t, "gpt-4-turbo", f.ChatCompletion.Model)
	assert.Equal(t, "fp_custom", f.ChatCompletion.SystemFingerprint)
	assert.Equal(t, 100, f.ChatCompletion.PromptTokens)
	assert.Equal(t, 50, f.ChatCompletion.CompletionTokens)

	// Other fixtures should remain defaults.
	defaults := fixture.Default()
	assert.Equal(t, defaults.Response, f.Response)
	assert.Equal(t, defaults.AudioTranscription, f.AudioTranscription)
}

func TestLoad_ResponsesFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "responses.json")

	data := `{
		"content": "Custom response text",
		"model": "gpt-4o-mini",
		"input_tokens": 200,
		"output_tokens": 75
	}`

	err := os.WriteFile(path, []byte(data), 0o644)
	require.NoError(t, err)

	cfg := &config.FixtureConfig{Responses: path}
	f, err := fixture.Load(cfg)

	require.NoError(t, err)
	assert.Equal(t, "Custom response text", f.Response.Content)
	assert.Equal(t, "gpt-4o-mini", f.Response.Model)
	assert.Equal(t, 200, f.Response.InputTokens)
	assert.Equal(t, 75, f.Response.OutputTokens)
}

func TestLoad_AudioTranscriptionFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audio.json")

	data := `{"text": "Custom transcription text"}`

	err := os.WriteFile(path, []byte(data), 0o644)
	require.NoError(t, err)

	cfg := &config.FixtureConfig{AudioTranscription: path}
	f, err := fixture.Load(cfg)

	require.NoError(t, err)
	assert.Equal(t, "Custom transcription text", f.AudioTranscription.Text)
}

func TestLoad_AllFixturesFromFiles(t *testing.T) {
	dir := t.TempDir()

	chatPath := filepath.Join(dir, "chat.json")
	respPath := filepath.Join(dir, "resp.json")
	audioPath := filepath.Join(dir, "audio.json")

	require.NoError(t, os.WriteFile(chatPath, []byte(`{
		"content": "Chat custom",
		"model": "gpt-custom",
		"system_fingerprint": "fp_all",
		"prompt_tokens": 10,
		"completion_tokens": 5
	}`), 0o644))

	require.NoError(t, os.WriteFile(respPath, []byte(`{
		"content": "Response custom",
		"model": "gpt-custom",
		"input_tokens": 15,
		"output_tokens": 8
	}`), 0o644))

	require.NoError(t, os.WriteFile(audioPath, []byte(`{
		"text": "Audio custom"
	}`), 0o644))

	cfg := &config.FixtureConfig{
		ChatCompletion:     chatPath,
		Responses:          respPath,
		AudioTranscription: audioPath,
	}
	f, err := fixture.Load(cfg)

	require.NoError(t, err)
	assert.Equal(t, "Chat custom", f.ChatCompletion.Content)
	assert.Equal(t, "Response custom", f.Response.Content)
	assert.Equal(t, "Audio custom", f.AudioTranscription.Text)
}

func TestLoad_FileNotFound(t *testing.T) {
	cfg := &config.FixtureConfig{ChatCompletion: "/nonexistent/path.json"}
	f, err := fixture.Load(cfg)

	require.Error(t, err)
	assert.Nil(t, f)
	assert.Contains(t, err.Error(), "chat completion fixture")
}

func TestLoad_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")

	err := os.WriteFile(path, []byte(`{not valid json}`), 0o644)
	require.NoError(t, err)

	cfg := &config.FixtureConfig{ChatCompletion: path}
	f, err := fixture.Load(cfg)

	require.Error(t, err)
	assert.Nil(t, f)
	assert.Contains(t, err.Error(), "parsing")
}

func TestLoad_ResponsesFileNotFound(t *testing.T) {
	cfg := &config.FixtureConfig{Responses: "/nonexistent/resp.json"}
	f, err := fixture.Load(cfg)

	require.Error(t, err)
	assert.Nil(t, f)
	assert.Contains(t, err.Error(), "responses fixture")
}

func TestLoad_AudioFileNotFound(t *testing.T) {
	cfg := &config.FixtureConfig{AudioTranscription: "/nonexistent/audio.json"}
	f, err := fixture.Load(cfg)

	require.Error(t, err)
	assert.Nil(t, f)
	assert.Contains(t, err.Error(), "audio transcription fixture")
}

func TestLoad_PartialJSON_KeepsDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "partial.json")

	// Only override content; other fields should keep their defaults.
	err := os.WriteFile(path, []byte(`{"content": "Partial content"}`), 0o644)
	require.NoError(t, err)

	cfg := &config.FixtureConfig{ChatCompletion: path}
	f, err := fixture.Load(cfg)

	require.NoError(t, err)
	assert.Equal(t, "Partial content", f.ChatCompletion.Content)

	// Fields not in the JSON retain their default values.
	defaults := fixture.Default()
	assert.Equal(t, defaults.ChatCompletion.Model, f.ChatCompletion.Model)
	assert.Equal(t, defaults.ChatCompletion.PromptTokens, f.ChatCompletion.PromptTokens)
	assert.Equal(t, defaults.ChatCompletion.SystemFingerprint, f.ChatCompletion.SystemFingerprint)
}
