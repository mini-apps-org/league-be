package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/nguyentanvinh/league-be/docs/swagger"
	"github.com/nguyentanvinh/league-be/internal/platform/config"
	"github.com/nguyentanvinh/league-be/internal/platform/db"
	"github.com/nguyentanvinh/league-be/internal/platform/logger"
	"github.com/nguyentanvinh/league-be/internal/platform/router"
	"github.com/rs/cors"
)

//	@title						League API
//	@version					1.0
//	@description				League service with Google sign-in and server-side sessions.
//	@BasePath					/api/v1
//
// host is deliberately unset so Swagger UI calls whichever origin served the
// page — localhost and the ngrok tunnel both work without regenerating.
//
//	@securityDefinitions.apikey	CSRFToken
//	@in							header
//	@name						X-CSRF-Token
//	@description				Echo the value of the `csrf` cookie. Required on every state-changing /api/v1 call.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()

	if err != nil {
		slog.Error("config: load failed", "err", err)
		os.Exit(1)
	}

	configured, levelErr := logger.New(cfg.Env, cfg.Logger.Level)
	slog.SetDefault(configured)
	if levelErr != nil {
		slog.Warn("logger: falling back to info", "err", levelErr)
	}

	pool, err := db.Connect(ctx, cfg)
	if err != nil {
		slog.Error("database: connect failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()
	slog.Info("database: connected", "target", cfg.RedactedDatabaseTarget())

	routes, err := (&router.Router{Config: cfg, Pool: pool}).NewRouter(ctx)
	if err != nil {
		slog.Error("router: build failed", "err", err)
		os.Exit(1)
	}

	// Exactly one allowed origin, with credentials: the session lives in a
	// cookie, which the browser sends cross-origin only when both are set.
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   cfg.Domains.AllowedOrigins,
		AllowedHeaders:   config.AllowHeadersAPI,
		AllowCredentials: true,
		AllowedMethods: []string{
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodDelete,
			http.MethodOptions,
			http.MethodHead,
		},
	})

	// TODO vincent: take these from cfg.Server once those fields are wired.
	server := &http.Server{
		Addr:              cfg.ServerPort(),
		Handler:           corsHandler.Handler(routes),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		slog.Info("server: listening", "addr", cfg.ServerPort(), "env", cfg.Env, "image", cfg.Image)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server: listen failed", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("server: shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server: shutdown failed", "err", err)
	}
}
