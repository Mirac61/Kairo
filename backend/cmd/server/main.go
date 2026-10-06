// Command server startet das Kairo-Backend.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"kairo/internal/api"
	"kairo/internal/config"
	"kairo/internal/repository"
	"kairo/internal/service"
	"kairo/internal/web"
)

// version wird per -ldflags "-X main.version=..." gesetzt.
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "kairo:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	token, err := api.LoadOrCreateToken(cfg.TokenPath)
	if err != nil {
		return err
	}
	db, err := repository.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	services := api.Services{
		Projects: service.NewProjectService(repository.NewProjectRepository(db), nil),
		Tasks:    service.NewTaskService(repository.NewTaskRepository(db), nil),
	}

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           api.NewRouter(cfg.Port, token, version, web.Handler(), services),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	log.Info("Kairo gestartet", "url", "http://"+config.BindHost+":"+strconv.Itoa(cfg.Port),
		"version", version, "db", cfg.DBPath, "timezone", cfg.Location.String())

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	log.Info("Beende ...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
