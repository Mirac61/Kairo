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
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"kairo/internal/api"
	"kairo/internal/config"
	"kairo/internal/launchd"
	"kairo/internal/realtime"
	"kairo/internal/repository"
	"kairo/internal/service"
	"kairo/internal/web"
)

// version wird per -ldflags "-X main.version=..." gesetzt.
var version = "dev"

func main() {
	if err := dispatch(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "kairo:", err)
		os.Exit(1)
	}
}

// dispatch führt "install", "uninstall" oder (ohne Argument) den Server aus.
func dispatch(args []string) error {
	if len(args) == 0 {
		return run()
	}
	switch args[0] {
	case "install":
		bin, err := os.Executable()
		if err != nil {
			return err
		}
		if bin, err = filepath.EvalSymlinks(bin); err != nil {
			return err
		}
		plist, err := launchd.Install(bin, os.Environ())
		if err != nil {
			return err
		}
		fmt.Println("LaunchAgent eingerichtet:", plist)
		fmt.Println("Das Backend startet jetzt bei jedem Login. Log:", filepath.Join(os.Getenv("HOME"), "Library", "Logs", "kairo.log"))
		return nil
	case "uninstall":
		plist, err := launchd.Uninstall()
		if err != nil {
			return err
		}
		fmt.Println("LaunchAgent entfernt:", plist)
		return nil
	default:
		return fmt.Errorf("unbekannter Befehl %q (erlaubt: install, uninstall oder ohne Argument den Server starten)", args[0])
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

	hub := realtime.NewHub()
	tasks := service.NewTaskService(repository.NewTaskRepository(db), nil)
	calendar := service.NewCalendarService(repository.NewCalendarRepository(db), cfg.Location, nil)
	habits := service.NewHabitService(repository.NewHabitRepository(db), cfg.Location, nil)
	timeTracking := service.NewTimeTrackingService(repository.NewTimeEntryRepository(db), nil)
	projects := service.NewProjectService(repository.NewProjectRepository(db), nil)
	for _, p := range []interface{ SetPublisher(service.Publisher) }{projects, tasks, calendar, habits, timeTracking} {
		p.SetPublisher(hub)
	}
	services := api.Services{
		Projects:  projects,
		Tasks:     tasks.WithTimerStopper(timeTracking),
		Calendar:  calendar,
		Habits:    habits,
		Resources: service.NewResourceService(repository.NewResourceRepository(db), nil),
		Time:      timeTracking,
		Today:     service.NewTodayService(tasks, calendar, habits, timeTracking, cfg.Location, nil),
		Hub:       hub,
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
