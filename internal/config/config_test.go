package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load("nonexistent.yaml")

	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.True(t, cfg.Auth.Enabled)
	assert.Empty(t, cfg.Auth.Token)
	assert.Empty(t, cfg.Fixtures.ChatCompletion)
	assert.Empty(t, cfg.Fixtures.Responses)
	assert.Empty(t, cfg.Fixtures.AudioTranscription)
}

func TestLoad_ValidYAML(t *testing.T) {
	content := `
server:
  host: "127.0.0.1"
  port: 9090
auth:
  enabled: false
  token: "test-token"
fixtures:
  chat_completion: "fixtures/chat.json"
  responses: "fixtures/resp.json"
  audio_transcription: "fixtures/audio.json"
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, err := Load(path)

	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, 9090, cfg.Server.Port)
	assert.False(t, cfg.Auth.Enabled)
	assert.Equal(t, "test-token", cfg.Auth.Token)
	assert.Equal(t, "fixtures/chat.json", cfg.Fixtures.ChatCompletion)
	assert.Equal(t, "fixtures/resp.json", cfg.Fixtures.Responses)
	assert.Equal(t, "fixtures/audio.json", cfg.Fixtures.AudioTranscription)
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.yaml")
	require.NoError(t, os.WriteFile(path, []byte("{{invalid"), 0o644))

	_, err := Load(path)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading config file")
}

func TestLoad_PartialYAML(t *testing.T) {
	content := `
server:
  port: 3000
`
	path := filepath.Join(t.TempDir(), "partial.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	cfg, err := Load(path)

	require.NoError(t, err)
	assert.Equal(t, "0.0.0.0", cfg.Server.Host, "unset fields keep defaults")
	assert.Equal(t, 3000, cfg.Server.Port, "set fields override defaults")
	assert.True(t, cfg.Auth.Enabled, "unset sections keep defaults")
}

func TestLoad_EnvOverrides(t *testing.T) {
	tests := []struct {
		name   string
		envs   map[string]string
		assert func(t *testing.T, cfg *Config)
	}{
		{
			name: "host override",
			envs: map[string]string{"MOCK_AI_SERVER_HOST": "localhost"},
			assert: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, "localhost", cfg.Server.Host)
			},
		},
		{
			name: "port override",
			envs: map[string]string{"MOCK_AI_SERVER_PORT": "9090"},
			assert: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, 9090, cfg.Server.Port)
			},
		},
		{
			name: "auth enabled false",
			envs: map[string]string{"MOCK_AI_AUTH_ENABLED": "false"},
			assert: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.False(t, cfg.Auth.Enabled)
			},
		},
		{
			name: "auth token override",
			envs: map[string]string{"MOCK_AI_AUTH_TOKEN": "sk-secret"},
			assert: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, "sk-secret", cfg.Auth.Token)
			},
		},
		{
			name: "multiple overrides",
			envs: map[string]string{
				"MOCK_AI_SERVER_HOST": "192.168.1.1",
				"MOCK_AI_SERVER_PORT": "3000",
				"MOCK_AI_AUTH_TOKEN":  "my-token",
			},
			assert: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, "192.168.1.1", cfg.Server.Host)
				assert.Equal(t, 3000, cfg.Server.Port)
				assert.Equal(t, "my-token", cfg.Auth.Token)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.envs {
				t.Setenv(k, v)
			}

			// Load with a nonexistent file so only defaults + env apply.
			cfg, err := Load("nonexistent.yaml")
			require.NoError(t, err)
			tt.assert(t, cfg)
		})
	}
}

func TestLoad_EnvOverridesYAML(t *testing.T) {
	content := `
server:
  host: "127.0.0.1"
  port: 9090
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))

	t.Setenv("MOCK_AI_SERVER_PORT", "4000")

	cfg, err := Load(path)

	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1", cfg.Server.Host, "YAML value kept when no env override")
	assert.Equal(t, 4000, cfg.Server.Port, "env var overrides YAML value")
}

func TestConfig_Addr(t *testing.T) {
	tests := []struct {
		name string
		host string
		port int
		want string
	}{
		{
			name: "default",
			host: "0.0.0.0",
			port: 8080,
			want: "0.0.0.0:8080",
		},
		{
			name: "localhost",
			host: "localhost",
			port: 3000,
			want: "localhost:3000",
		},
		{
			name: "empty host",
			host: "",
			port: 8080,
			want: ":8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Server: ServerConfig{
					Host: tt.host,
					Port: tt.port,
				},
			}
			assert.Equal(t, tt.want, cfg.Addr())
		})
	}
}
