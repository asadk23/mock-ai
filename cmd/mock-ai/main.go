package main

import (
	"net/http"
	"os"
	"time"

	"github.com/rs/zerolog"

	"github.com/asadk23/mock-ai/internal/config"
	"github.com/asadk23/mock-ai/internal/fixture"
	"github.com/asadk23/mock-ai/internal/server"
	"github.com/asadk23/mock-ai/internal/store"
)

func main() {
	logger := zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}).With().Timestamp().Logger()

	cfg, err := config.Load("config.yaml")
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load config")
	}

	logger.Info().
		Str("host", cfg.Server.Host).
		Int("port", cfg.Server.Port).
		Bool("auth_enabled", cfg.Auth.Enabled).
		Msg("configuration loaded")

	fixtures, err := fixture.Load(&cfg.Fixtures)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load fixtures")
	}

	memStore := store.NewMemory()

	r := server.NewRouter(server.Options{
		Store:       memStore,
		Fixtures:    fixtures,
		Logger:      &logger,
		AuthEnabled: cfg.Auth.Enabled,
		AuthToken:   cfg.Auth.Token,
	})

	addr := cfg.Addr()
	logger.Info().Str("addr", addr).Msg("starting mock-ai server")

	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	if err := srv.ListenAndServe(); err != nil {
		logger.Fatal().Err(err).Msg("server failed")
	}
}
