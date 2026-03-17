# ADR-0001: Mock OpenAI Server Architecture

## Status

Accepted

## Date

2026-03-13

## Context

We need a mock server that replicates the OpenAI API spec for local development
and testing. This allows developers to build and test integrations against
OpenAI's API without incurring costs, hitting rate limits, or requiring network
access.

The OpenAI API (v2.3.0) contains 161 endpoints across 30 categories. We are
scoping the initial implementation to the most commonly used API surfaces:
Chat Completions, Responses, and Audio.

## Decision

### Scope: Endpoints

We will implement 15 endpoints across 3 API groups. Models and all other
endpoints are out of scope for the initial version.

#### Chat Completions (6 endpoints)

| Method | Path                                              | Description              |
|--------|---------------------------------------------------|--------------------------|
| POST   | `/v1/chat/completions`                            | Create completion (+SSE) |
| GET    | `/v1/chat/completions`                            | List completions         |
| GET    | `/v1/chat/completions/{completion_id}`            | Retrieve completion      |
| POST   | `/v1/chat/completions/{completion_id}`            | Update completion        |
| DELETE | `/v1/chat/completions/{completion_id}`            | Delete completion        |
| GET    | `/v1/chat/completions/{completion_id}/messages`   | List messages            |

#### Responses API (6 endpoints)

| Method | Path                                        | Description        |
|--------|---------------------------------------------|--------------------|
| POST   | `/v1/responses`                             | Create response (+SSE) |
| GET    | `/v1/responses/{response_id}`               | Retrieve response  |
| DELETE | `/v1/responses/{response_id}`               | Delete response    |
| POST   | `/v1/responses/{response_id}/cancel`        | Cancel response    |
| GET    | `/v1/responses/{response_id}/input_items`   | List input items   |
| POST   | `/v1/responses/count_tokens`                | Count input tokens |

#### Audio (3 endpoints)

| Method | Path                        | Description          |
|--------|-----------------------------|----------------------|
| POST   | `/v1/audio/speech`          | Create speech (TTS)  |
| POST   | `/v1/audio/transcriptions`  | Create transcription |
| POST   | `/v1/audio/translations`    | Create translation   |

### Language & Runtime

- **Language**: Go 1.23+
- **Module path**: `github.com/asadk23/mock-ai`

### Libraries

| Library                        | Purpose              | Rationale                                                    |
|--------------------------------|----------------------|--------------------------------------------------------------|
| `github.com/go-chi/chi/v5`    | HTTP router          | Lightweight, idiomatic, stdlib-compatible, strong middleware  |
| `github.com/google/uuid`      | ID generation        | Generate OpenAI-style IDs (`chatcmpl-*`, `resp_*`)          |
| `github.com/rs/zerolog`       | Structured logging   | Zero-allocation, fast, JSON output                           |
| `github.com/spf13/viper`      | Config management    | Unified YAML file, env var, and default value handling        |
| `github.com/stretchr/testify` | Test assertions      | Clean assertions (`assert`, `require`) for tests             |

No database. No ORM. All state is in-memory and resets on restart.

### Project Structure

```
mock-ai/
├── cmd/
│   └── mock-ai/
│       └── main.go              # Entrypoint: config, server startup
├── internal/
│   ├── config/
│   │   └── config.go            # Config struct, YAML loading, defaults
│   ├── middleware/
│   │   ├── auth.go              # Bearer token validation
│   │   ├── logging.go           # Request/response logging
│   │   └── headers.go          # OpenAI-specific response headers
│   ├── api/
│   │   └── api.go               # HTTP response helpers (WriteJSON, WriteError)
│   ├── model/
│   │   ├── common.go            # Shared types (Usage, Error, constants)
│   │   ├── chat.go              # Chat request/response types
│   │   ├── responses.go         # Responses API types
│   │   ├── audio.go             # Audio request/response types
│   │   └── streaming.go         # SSE event types
│   ├── handler/
│   │   ├── chat.go              # Chat Completions endpoints
│   │   ├── responses.go         # Responses API endpoints
│   │   └── audio.go             # Audio endpoints
│   ├── store/
│   │   └── memory.go            # In-memory store for state
│   ├── server/
│   │   └── router.go            # Full router assembly (NewRouter)
│   ├── streaming/
│   │   └── sse.go               # SSE writer utility
│   └── fixture/
│       ├── loader.go            # Load fixtures from files
│       └── defaults.go          # Compiled-in default responses
├── fixtures/
│   ├── chat_completion.json     # Default chat completion fixture
│   ├── responses.json           # Default responses API fixture
│   └── audio_transcription.json # Default audio response fixture
├── test/
│   └── integration_test.go      # Full-stack integration tests
├── docs/
│   └── ADR-0001-mock-openai-server-architecture.md
├── Makefile
├── AGENTS.md
├── go.mod
└── go.sum
```

### Response Strategy: Configurable Fixtures

1. **Default fixtures** are compiled into the binary (`fixture/defaults.go`) so
   the server works out of the box with zero configuration.
2. **Custom fixtures** can be loaded from JSON files on disk, configured via a
   YAML config file.
3. **Per-endpoint overrides** allow different responses for different endpoints.
4. Responses use **dynamic fields** where appropriate: generated IDs (`chatcmpl-*`,
   `resp_*`), current timestamps, and computed token usage.

### Streaming (SSE)

Both Chat Completions and Responses API support streaming via Server-Sent
Events (SSE). Both use the same HTTP headers and termination sentinel, but
differ in their SSE event format.

#### Common

- **HTTP Headers**: `Content-Type: text/event-stream`, `Cache-Control: no-cache`,
  `Connection: keep-alive` (plus the standard OpenAI response headers).
- **Stream Termination**: Both APIs terminate with `data: [DONE]\n\n`.
- **Flushing**: The SSE writer flushes after each event to ensure real-time
  delivery to the client.

#### Chat Completions

`POST /v1/chat/completions` with `"stream": true` uses **bare `data:` lines
only** -- no named `event:` field. Each line contains a `ChatCompletionChunk`
JSON object with `object: "chat.completion.chunk"`:

```
data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","created":1694268190,"model":"gpt-4o","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}

data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","created":1694268190,"model":"gpt-4o","choices":[{"index":0,"delta":{"content":"Hello"},"finish_reason":null}]}

data: {"id":"chatcmpl-abc","object":"chat.completion.chunk","created":1694268190,"model":"gpt-4o","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]
```

When `stream_options.include_usage` is `true`, an additional chunk with
`choices: []` (empty) and a populated `usage` object is sent immediately
before `data: [DONE]`.

#### Responses API

`POST /v1/responses` with `"stream": true` uses the **full SSE spec with
named events** -- each message has both an `event:` line and a `data:` line.
The JSON data also contains a `type` field matching the event name, plus a
monotonically increasing `sequence_number`:

```
event: response.created
data: {"type":"response.created","response":{...},"sequence_number":0}

event: response.in_progress
data: {"type":"response.in_progress","response":{...},"sequence_number":1}

event: response.output_item.added
data: {"type":"response.output_item.added","item":{...},"output_index":0,"sequence_number":2}

event: response.content_part.added
data: {"type":"response.content_part.added","part":{...},"content_index":0,"item_id":"msg_abc","output_index":0,"sequence_number":3}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"Hello","content_index":0,"item_id":"msg_abc","output_index":0,"sequence_number":4}

event: response.output_text.done
data: {"type":"response.output_text.done","text":"Hello!","content_index":0,"item_id":"msg_abc","output_index":0,"sequence_number":5}

event: response.content_part.done
data: {"type":"response.content_part.done","part":{...},"content_index":0,"item_id":"msg_abc","output_index":0,"sequence_number":6}

event: response.output_item.done
data: {"type":"response.output_item.done","item":{...},"output_index":0,"sequence_number":7}

event: response.completed
data: {"type":"response.completed","response":{...},"sequence_number":8}

data: [DONE]
```

Core event types we implement (the real API has 40+ event types for built-in
tools like web_search, code_interpreter, MCP, image_gen, etc. -- those are
out of scope for this mock):

| Event Type | Description |
|---|---|
| `response.created` | Response object created |
| `response.in_progress` | Generation started |
| `response.completed` | Generation finished |
| `response.failed` | Generation failed |
| `response.incomplete` | Generation stopped early (e.g. max tokens) |
| `response.cancelled` | Response was canceled |
| `response.output_item.added` | Output item added |
| `response.output_item.done` | Output item finished |
| `response.content_part.added` | Content part added to an output item |
| `response.content_part.done` | Content part finished |
| `response.output_text.delta` | Incremental text content |
| `response.output_text.done` | Text output finished |
| `response.refusal.delta` | Incremental refusal text |
| `response.refusal.done` | Refusal finished |
| `response.function_call_arguments.delta` | Incremental function call args |
| `response.function_call_arguments.done` | Function call args finished |
| `error` | Error during streaming |

### Authentication

- Middleware checks for `Authorization: Bearer <token>` header.
- By default, any non-empty token is accepted.
- A specific required token can be set via config for stricter testing.
- Missing/malformed auth returns `401` in OpenAI's error format.

### Error Responses

All errors match OpenAI's exact format:

```json
{
  "error": {
    "message": "Invalid API key provided.",
    "type": "invalid_request_error",
    "param": null,
    "code": "invalid_api_key"
  }
}
```

### OpenAI-Compatible Headers

Every response includes:
- `x-request-id` -- unique per request (UUID)
- `openai-organization` -- hardcoded org ID
- `openai-processing-ms` -- elapsed time
- `openai-version` -- `2020-10-01` (matches real API)

### Configuration

```yaml
server:
  port: 8080
  host: "0.0.0.0"

auth:
  enabled: true
  token: ""  # empty = accept any Bearer token

fixtures:
  chat_completion: "fixtures/chat_completion.json"
  responses: "fixtures/responses.json"
  audio_transcription: "fixtures/audio_transcription.json"
```

### Testing

- **Unit tests**: Per-handler tests using `httptest.NewRecorder()`.
- **Integration tests**: Full server tests in `test/` making real HTTP calls.
- **Streaming tests**: Verify SSE event format, chunking, `[DONE]` termination.
- Framework: `testify/assert` and `testify/require`.

### Build Commands

```
make build          # go build -o bin/mock-ai ./cmd/mock-ai
make run            # go run ./cmd/mock-ai
make test           # go test ./...
make test-verbose   # go test -v ./...
make lint           # golangci-lint run ./...
make fmt            # gofmt -w .
```

Single test: `go test -run TestChatCompletionCreate ./test/`

## Consequences

### Positive

- Developers can test OpenAI integrations offline and without API costs.
- Configurable fixtures allow testing specific response scenarios (errors,
  edge cases, different model outputs).
- SSE streaming support enables testing real streaming client code.
- In-memory store keeps the implementation simple and fast.
- Extensible structure makes it straightforward to add more endpoint groups
  (Models, Files, Embeddings, etc.) later.

### Negative

- In-memory state is lost on restart (acceptable for a mock server).
- Request validation covers required fields and basic type checking, but does
  not enforce the full OpenAI JSON schema (e.g., enum value ranges, nested
  object constraints beyond required fields).
- No support for Realtime (WebSocket) API in initial scope.

### Risks

- OpenAI may change their API response format. Fixtures and types will need
  updating to stay current.
