// Package server provides the fully-wired HTTP router for mock-ai.
package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"

	"github.com/asadk23/mock-ai/internal/fixture"
	"github.com/asadk23/mock-ai/internal/handler"
	"github.com/asadk23/mock-ai/internal/middleware"
	"github.com/asadk23/mock-ai/internal/store"
)

// Options configures the router assembly.
type Options struct {
	Store    store.Store
	Fixtures *fixture.Fixtures
	Logger   *zerolog.Logger

	// Auth controls whether authentication middleware is enabled
	// and what token to require.
	AuthEnabled bool
	AuthToken   string
}

// NewRouter creates a fully-wired chi.Router with all middleware and handlers
// mounted. This is used by both the main server and integration tests.
func NewRouter(opts Options) chi.Router {
	r := chi.NewRouter()

	// Global middleware stack (order matters: logging -> auth -> OpenAI headers).
	r.Use(middleware.Logging(opts.Logger))
	r.Use(middleware.Auth(opts.AuthEnabled, opts.AuthToken))
	r.Use(middleware.OpenAIHeaders)

	// Health check (no auth required — but auth middleware runs first;
	// if auth is enabled, health requires a token too, which matches
	// OpenAI behavior where all endpoints require auth).
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Handler constructors.
	chatHandler := handler.NewChatHandler(opts.Store, opts.Fixtures, opts.Logger)
	responseHandler := handler.NewResponseHandler(opts.Store, opts.Fixtures, opts.Logger)
	audioHandler := handler.NewAudioHandler(opts.Fixtures, opts.Logger)

	// Mount API routes under /v1.
	r.Route("/v1", func(r chi.Router) {
		r.Mount("/chat/completions", chatHandler.Routes())
		r.Mount("/responses", responseHandler.Routes())
		r.Mount("/audio", audioHandler.Routes())
	})

	return r
}
