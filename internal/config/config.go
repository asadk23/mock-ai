// Package config provides configuration loading for the mock-ai server.
// Configuration is loaded from a YAML file with environment variable overrides
// using Viper.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// Config holds the complete server configuration.
type Config struct {
	Server   ServerConfig  `mapstructure:"server"`
	Auth     AuthConfig    `mapstructure:"auth"`
	Fixtures FixtureConfig `mapstructure:"fixtures"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Token   string `mapstructure:"token"`
}

// FixtureConfig holds file paths for fixture overrides.
type FixtureConfig struct {
	ChatCompletion     string `mapstructure:"chat_completion"`
	Responses          string `mapstructure:"responses"`
	AudioTranscription string `mapstructure:"audio_transcription"`
}

const (
	_defaultHost = "0.0.0.0"
	_defaultPort = 8080
)

// Load reads configuration from the given YAML file path, applies defaults,
// and overlays environment variable overrides. If the file does not exist,
// defaults and env vars are still applied without error.
//
// Environment variables use the MOCK_AI prefix with underscores replacing dots:
//   - MOCK_AI_SERVER_HOST
//   - MOCK_AI_SERVER_PORT
//   - MOCK_AI_AUTH_ENABLED
//   - MOCK_AI_AUTH_TOKEN
//   - MOCK_AI_FIXTURES_CHAT_COMPLETION
//   - MOCK_AI_FIXTURES_RESPONSES
//   - MOCK_AI_FIXTURES_AUDIO_TRANSCRIPTION
func Load(path string) (*Config, error) {
	v := viper.New()

	// Defaults.
	v.SetDefault("server.host", _defaultHost)
	v.SetDefault("server.port", _defaultPort)
	v.SetDefault("auth.enabled", true)
	v.SetDefault("auth.token", "")
	v.SetDefault("fixtures.chat_completion", "")
	v.SetDefault("fixtures.responses", "")
	v.SetDefault("fixtures.audio_transcription", "")

	// YAML file.
	v.SetConfigFile(path)

	if err := v.ReadInConfig(); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			// Viper wraps file-not-found differently depending on the OS, so
			// also check for its own ConfigFileNotFoundError.
			var configNotFound viper.ConfigFileNotFoundError
			if !errors.As(err, &configNotFound) {
				return nil, fmt.Errorf("reading config file: %w", err)
			}
		}
	}

	// Environment variable overrides.
	v.SetEnvPrefix("MOCK_AI")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshalling config: %w", err)
	}

	return &cfg, nil
}

// Addr returns the server listen address in "host:port" format.
func (c *Config) Addr() string {
	return c.Server.Host + ":" + strconv.Itoa(c.Server.Port)
}
