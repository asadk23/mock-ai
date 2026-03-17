# mock-ai

A Go mock server that replicates the OpenAI API for local development and testing. It provides configurable fixture responses, SSE streaming support, and in-memory state management -- no external dependencies or API keys required.

> This project was built entirely with [OpenCode](https://opencode.ai).

## Features

- **Chat Completions API** -- Create, list, get, update, delete completions and list messages
- **Responses API** -- Create, get, delete responses, cancel in-progress responses, list input items, count tokens
- **Audio API** -- Text-to-speech, transcription, and translation
- **SSE Streaming** -- Chat Completions uses bare `data:` lines; Responses API uses named `event:` + `data:` pairs, both matching the real OpenAI wire format
- **Configurable Fixtures** -- Override default responses with custom JSON fixtures
- **Bearer Token Auth** -- Optional authentication matching OpenAI's format
- **OpenAI-Compatible Headers** -- `x-request-id`, `openai-organization`, `openai-processing-ms`, `openai-version`
- **OpenAI Error Format** -- All errors follow OpenAI's `{"error": {...}}` structure
- **Zero Configuration** -- Runs with sensible defaults out of the box

## Quick Start

### Prerequisites

- Go 1.23+

### Build and Run

```bash
# Build
make build

# Run (defaults to 0.0.0.0:8080)
make run

# Or run directly
go run ./cmd/mock-ai
```

### Verify

```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

## Configuration

Configuration is loaded from `config.yaml` with environment variable overrides.

### config.yaml

```yaml
server:
  host: "0.0.0.0"
  port: 8080

auth:
  enabled: true
  token: "sk-mock-token"  # leave empty to accept any Bearer token

fixtures:
  chat_completion: ""      # path to custom chat completion fixture JSON
  responses: ""            # path to custom responses fixture JSON
  audio_transcription: ""  # path to custom audio transcription fixture JSON
```

### Environment Variables

All settings can be overridden with environment variables using the `MOCK_AI` prefix:

| Variable | Description | Default |
|----------|-------------|---------|
| `MOCK_AI_SERVER_HOST` | Listen host | `0.0.0.0` |
| `MOCK_AI_SERVER_PORT` | Listen port | `8080` |
| `MOCK_AI_AUTH_ENABLED` | Enable Bearer token auth | `true` |
| `MOCK_AI_AUTH_TOKEN` | Required token (empty = any token accepted) | `""` |
| `MOCK_AI_FIXTURES_CHAT_COMPLETION` | Custom chat completion fixture path | `""` |
| `MOCK_AI_FIXTURES_RESPONSES` | Custom responses fixture path | `""` |
| `MOCK_AI_FIXTURES_AUDIO_TRANSCRIPTION` | Custom audio transcription fixture path | `""` |

## API Endpoints

### Chat Completions

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/chat/completions` | Create a chat completion |
| `GET` | `/v1/chat/completions` | List chat completions |
| `GET` | `/v1/chat/completions/{id}` | Get a chat completion |
| `POST` | `/v1/chat/completions/{id}` | Update a chat completion |
| `DELETE` | `/v1/chat/completions/{id}` | Delete a chat completion |
| `GET` | `/v1/chat/completions/{id}/messages` | List messages for a completion |

### Responses

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/responses` | Create a response |
| `GET` | `/v1/responses/{id}` | Get a response |
| `DELETE` | `/v1/responses/{id}` | Delete a response |
| `POST` | `/v1/responses/{id}/cancel` | Cancel a response |
| `GET` | `/v1/responses/{id}/input_items` | List input items |
| `POST` | `/v1/responses/count_tokens` | Count tokens |

### Audio

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/audio/speech` | Generate speech from text |
| `POST` | `/v1/audio/transcriptions` | Transcribe audio to text |
| `POST` | `/v1/audio/translations` | Translate audio to English text |

### Health

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/health` | Health check |

## Development

```bash
# Run all tests
make test

# Run tests with verbose output
make test-verbose

# Run a specific test
go test -run TestChatCompletionCreate ./test/

# Lint
make lint

# Format
make fmt

# Clean build artifacts
make clean
```

## Project Structure

```
cmd/mock-ai/main.go         -- Entrypoint: config loading, server startup
internal/config/             -- Config struct, YAML loading, defaults
internal/middleware/         -- HTTP middleware (auth, logging, OpenAI headers)
internal/handler/            -- Route handlers grouped by API (chat, responses, audio)
internal/model/              -- Request/response type definitions
internal/store/              -- In-memory state store
internal/streaming/          -- SSE writer utility
internal/api/                -- HTTP response helpers (WriteJSON, WriteError)
internal/server/             -- Router assembly and wiring
internal/fixture/            -- Fixture loading and compiled-in defaults
fixtures/                    -- JSON fixture files for mock responses
test/                        -- Integration tests
docs/                        -- ADRs and documentation
```

## License

MIT
