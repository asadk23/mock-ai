# AGENTS.md

## Project Overview

mock-ai is a Go mock server that replicates the OpenAI API spec for local
development and testing. It covers Chat Completions, Responses API, and Audio
endpoints with configurable fixtures and SSE streaming support.

See `docs/ADR-0001-mock-openai-server-architecture.md` for full design decisions.

## Build / Run / Test Commands

```bash
# Build
make build                  # go build -o bin/mock-ai ./cmd/mock-ai

# Run
make run                    # go run ./cmd/mock-ai

# Test - all
make test                   # go test ./...
make test-verbose           # go test -v ./...

# Test - single test by name
go test -run TestChatCompletionCreate ./test/
go test -run TestName ./internal/handler/

# Test - single file (run all tests in a file's package, filter by names in that file)
go test -v -run "TestFunc1|TestFunc2" ./internal/handler/

# Lint
make lint                   # golangci-lint run ./...

# Format
make fmt                    # gofmt -w .
```

## Project Structure

```
cmd/mock-ai/main.go         — Entrypoint: config loading, server startup
internal/config/             — Config struct, YAML loading, defaults
internal/middleware/         — HTTP middleware (auth, logging, OpenAI headers)
internal/handler/            — Route handlers grouped by API (chat, responses, audio)
internal/model/              — Request/response type definitions
internal/store/              — In-memory state store
internal/streaming/          — SSE writer utility
internal/fixture/            — Fixture loading and compiled-in defaults
fixtures/                    — JSON fixture files for mock responses
test/                        — Integration tests
docs/                        — ADRs and documentation
```

## Code Style Guidelines

This project follows the [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md)
as its foundational code style reference. The rules below supplement or
override it with project-specific conventions.

Key points from the Uber guide that are especially relevant here:

- Avoid `init()` — use explicit initialization in `main()`.
- Don't panic — return errors and let callers decide.
- Handle errors once — don't log and return the same error.
- Use `%w` for error wrapping to support `errors.Is`/`errors.As`.
- Reduce nesting — handle errors early, return early.
- Prefer `strconv` over `fmt` for type conversions.
- Use table-driven tests with subtests.
- Prefix unexported globals with `_` (except error values with `Err`/`err`).
- Use field names when initializing structs.
- Specify container capacity where possible (`make([]T, 0, n)`).
- Use functional options pattern for constructors with optional arguments.
- Avoid embedding types in public structs.
- Use `defer` for cleanup (locks, files, connections).
- Verify interface compliance at compile time (`var _ Interface = (*Type)(nil)`).

### Go Version

Go 1.23+. Use modern stdlib features (e.g., enhanced `net/http` routing patterns
from 1.22+, but we use chi for routing).

### Imports

Use three groups separated by blank lines, in this order:

```go
import (
    "context"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/rs/zerolog"

    "github.com/asadk23/mock-ai/internal/model"
    "github.com/asadk23/mock-ai/internal/store"
)
```

1. Standard library
2. Third-party packages
3. Internal packages

Use `goimports` to auto-sort. Never use dot imports. Never use blank imports
except for side-effect packages (e.g., database drivers) with a comment.

### Formatting

- Run `gofmt` (or `goimports`) on all files. No exceptions.
- No manual alignment of struct fields — let `gofmt` handle it.
- Max line length is not enforced by Go tooling, but keep lines readable
  (~100-120 chars). Break long function signatures after the opening paren.

### Naming Conventions

- **Packages**: lowercase, single word, no underscores (`handler`, `model`, `store`).
- **Files**: lowercase, underscores for multi-word (`chat_test.go`, `sse.go`).
- **Exported types**: `PascalCase` (`ChatCompletionRequest`, `SSEWriter`).
- **Unexported identifiers**: `camelCase` (`writeJSON`, `newRequestID`).
- **Constants**: `PascalCase` for exported, `camelCase` for unexported. No `ALL_CAPS`.
- **Interfaces**: name by behavior, not `I`-prefix (`Store`, `Flusher`, not `IStore`).
- **Acronyms**: keep consistent casing (`ID`, `SSE`, `HTTP`, `URL`, `JSON`, `API`).
  Use `ID` not `Id`, `HTTP` not `Http` in exported names. In unexported names
  at the start: `httpClient`, `sseWriter`.
- **Test functions**: `TestSubject_Scenario` (e.g., `TestChatHandler_StreamingResponse`).

### Types

- Define request/response structs in `internal/model/` with JSON tags on every field.
- Use `json:"field_name"` (snake_case) to match OpenAI's API format.
- Use `json:"field,omitempty"` for optional fields.
- Use pointer types (`*string`, `*int`) for nullable/optional fields in requests.
- Use concrete types (not `interface{}`) wherever the shape is known.
- Avoid `any` / `interface{}` except for genuinely dynamic data.

```go
type ChatCompletionRequest struct {
    Model    string           `json:"model"`
    Messages []ChatMessage    `json:"messages"`
    Stream   *bool            `json:"stream,omitempty"`
    MaxTokens *int            `json:"max_tokens,omitempty"`
}
```

### Error Handling

- Always check errors immediately. Never discard errors with `_`.
- Use OpenAI's error response format for all HTTP errors:
  ```go
  type APIError struct {
      Error ErrorBody `json:"error"`
  }
  type ErrorBody struct {
      Message string  `json:"message"`
      Type    string  `json:"type"`
      Param   *string `json:"param"`
      Code    string  `json:"code"`
  }
  ```
- Use helper functions to write error responses (`writeError(w, status, msg, code)`).
- In handlers, return after writing an error — do not continue processing.
- Log errors with zerolog at the appropriate level (`Warn` for client errors,
  `Error` for server errors).

### HTTP Handlers

- Handlers are methods on a struct that holds dependencies (store, config, logger).
- Register routes in a function that returns a `chi.Router`.
- Parse request bodies with `json.NewDecoder(r.Body).Decode(&req)`.
- Write responses with a `writeJSON(w, status, data)` helper.
- Set `Content-Type: application/json` on all JSON responses.
- For streaming, set `Content-Type: text/event-stream` and flush after each event.

### Testing

- Use `testify/assert` for soft assertions, `testify/require` for fatal assertions.
- Use `httptest.NewServer` or `httptest.NewRecorder` for handler tests.
- Test files live next to the code they test (`handler/chat_test.go`) for unit tests.
- Integration tests live in `test/` and spin up the full router.
- Name tests `TestSubject_Scenario` for clarity in `go test -v` output.
- Every handler must have tests for: success case, error cases (bad input, not found),
  and streaming (where applicable).

### Middleware

- Middleware follows the `func(next http.Handler) http.Handler` pattern.
- Order: logging -> auth -> OpenAI headers -> route handler.
- Auth middleware returns 401 for missing/invalid tokens using OpenAI error format.
- Headers middleware adds `x-request-id`, `openai-organization`,
  `openai-processing-ms`, `openai-version` to every response.

### Configuration

- Config is loaded from `config.yaml` (or env vars) at startup.
- Use a single `Config` struct in `internal/config/`.
- Provide sensible defaults so the server runs with zero configuration.
- Environment variables override file config (e.g., `MOCK_AI_PORT=9090`).

### Dependencies

Only these external dependencies are approved (see ADR-0001):

- `github.com/go-chi/chi/v5` — router
- `github.com/google/uuid` — ID generation
- `github.com/rs/zerolog` — logging
- `github.com/spf13/viper` — config management (YAML + env vars)
- `gopkg.in/yaml.v3` — config parsing (viper dependency)
- `github.com/stretchr/testify` — testing (test-only dependency)

Adding new dependencies requires discussion and a new ADR.

### Git & Commits

- Keep commits atomic — one logical change per commit.
- Do not commit generated files, binaries, or IDE configs.
- **Squash-merge** PRs so the PR title becomes the commit message on `main`.

#### Conventional Commits (required)

All commit messages and PR titles **must** follow
[Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/).
release-please parses these to decide version bumps and changelog entries.

| Prefix | When to use | Triggers release? | SemVer bump |
|---|---|---|---|
| `feat:` | New feature or endpoint | Yes | MINOR |
| `fix:` | Bug fix | Yes | PATCH |
| `feat!:` / `fix!:` / `refactor!:` | Breaking change (append `!`) | Yes | MAJOR |
| `docs:` | Documentation only | No | -- |
| `test:` | Adding/fixing tests | No | -- |
| `refactor:` | Code restructuring, no behavior change | No | -- |
| `ci:` | CI/CD changes | No | -- |
| `chore:` | Dependencies, tooling, config | No | -- |
| `perf:` | Performance improvement | No | -- |
| `build:` | Build system changes | No | -- |

<!-- AGENT INSTRUCTION: If it is not clear whether a change is feat, fix,
     refactor, or breaking — ASK the user before committing. Do not guess
     the commit prefix. -->

#### release-please

Automated versioning via [release-please-action v4](https://github.com/googleapis/release-please-action).
Config lives in `release-please-config.json` and `.release-please-manifest.json`.
On push to `main`, release-please opens/updates a Release PR. Merging that PR
creates the git tag (`vX.Y.Z`) and GitHub Release automatically.
