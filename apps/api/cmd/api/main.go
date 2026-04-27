package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/belchch/rms_platform/api/internal/config"
	"github.com/belchch/rms_platform/api/internal/db"
	authhandler "github.com/belchch/rms_platform/api/internal/handler/auth"
	photoshandler "github.com/belchch/rms_platform/api/internal/handler/photos"
	synchandler "github.com/belchch/rms_platform/api/internal/handler/sync"
	"github.com/belchch/rms_platform/api/internal/middleware"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatal().Err(err).Msg("database ping failed")
	}
	log.Info().Msg("database connected")

	router := chi.NewRouter()
	router.Use(middleware.Recover)
	router.Use(middleware.Logger)

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	queries := db.New(pool)

	api := humachi.New(router, huma.DefaultConfig("RMS Platform API", "0.1.0"))

	authhandler.Register(api, queries, pool)
	synchandler.Register(api, queries, pool)
	photoshandler.Register(api, queries, pool)

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Info().Str("addr", addr).Msg("starting server")

	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal().Err(err).Msg("server error")
	}
}
