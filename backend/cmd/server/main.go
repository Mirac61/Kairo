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
	"sync/atomic"
	"syscall"
	"time"

	"kairo/internal/api"
	"kairo/internal/autostart"
	"kairo/internal/config"
	"kairo/internal/notes"
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
		if err := run(); !errors.Is(err, errRestart) {
			return err
		}
		// Neustart aus der WebUI: dasselbe Binary mit denselben Argumenten und
		// derselben Umgebung liest config.json neu.
		bin, err := os.Executable()
		if err != nil {
			return err
		}
		return restartSelf(bin)
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
		path, log, err := autostart.Install(bin, os.Environ())
		if err != nil {
			return err
		}
		fmt.Println("Autostart eingerichtet:", path)
		fmt.Println("Das Backend läuft jetzt und startet bei jedem Login. Log:", log)
		return nil
	case "uninstall":
		path, err := autostart.Uninstall()
		if err != nil {
			return err
		}
		fmt.Println("Autostart entfernt:", path)
		return nil
	case "backup":
		return backup(args[1:])
	default:
		return fmt.Errorf("unbekannter Befehl %q (erlaubt: install, uninstall, backup [datei] oder ohne Argument den Server starten)", args[0])
	}
}

// backup kopiert die Datenbank nach args[0] (Standard: backups/ neben der Datenbank).
func backup(args []string) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	dest := filepath.Join(backupDir(cfg), "kairo-"+time.Now().Format("20060102-150405")+".db")
	if len(args) > 0 {
		dest = args[0]
	}
	ctx := context.Background()
	db, err := repository.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := repository.Backup(ctx, db, dest); err != nil {
		return err
	}
	fmt.Println("Backup geschrieben:", dest)
	return nil
}

// backupDir ist das Verzeichnis für Backups: backups/ neben der Datenbank.
func backupDir(cfg config.Config) string { return filepath.Join(filepath.Dir(cfg.DBPath), "backups") }

// errRestart beendet run, damit dispatch den Prozess neu startet.
var errRestart = errors.New("Neustart")

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	var restart atomic.Bool

	token, err := api.LoadOrCreateToken(cfg.TokenPath)
	if err != nil {
		return err
	}
	db, err := repository.OpenWithBackup(ctx, cfg.DBPath, backupDir(cfg))
	if err != nil {
		return err
	}
	defer db.Close()

	notesStore, err := notes.Open(cfg.NotesDir)
	if err != nil {
		return err
	}
	defer notesStore.Close()

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
		Today:     service.NewTodayService(tasks, calendar, habits, timeTracking, cfg.Location, nil).WithWorkWindow(cfg.WorkStart, cfg.WorkEnd),
		Review:    service.NewReviewService(tasks, projects, habits, calendar, timeTracking, cfg.Location, nil),
		Notes:     notesStore,
		Settings:  &api.Settings{Running: cfg, Getenv: os.Getenv, Restart: func() { restart.Store(true); stop() }},
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
		"version", version, "db", cfg.DBPath, "notes", cfg.NotesDir, "timezone", cfg.Location.String())

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
	if restart.Load() {
		return errRestart
	}
	return nil
}
