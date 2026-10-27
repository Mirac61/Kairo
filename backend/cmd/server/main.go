// Command server startet das Kairo-Backend.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"kairo/internal/api"
	"kairo/internal/auth"
	"kairo/internal/autostart"
	"kairo/internal/config"
	"kairo/internal/domain"
	"kairo/internal/github"
	"kairo/internal/importer"
	"kairo/internal/notes"
	"kairo/internal/realtime"
	"kairo/internal/reminder"
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

// dispatch führt "install", "uninstall", "backup", "restore" oder (ohne Argument) den Server aus.
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
	case "restore":
		return restore(args[1:])
	case "import":
		return importJSON(args[1:])
	default:
		return fmt.Errorf("unbekannter Befehl %q (erlaubt: install, uninstall, backup [datei], restore <datei>, import <datei> oder ohne Argument den Server starten)", args[0])
	}
}

// restore stellt die Datenbank aus der Backup-Datei args[0] wieder her.
func restore(args []string) error {
	force := false
	var files []string
	for _, a := range args {
		if a == "--force" {
			force = true
		} else {
			files = append(files, a)
		}
	}
	if len(files) != 1 {
		return errors.New("Aufruf: kairo restore [--force] <datei>")
	}
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	aside, err := restoreDB(cfg.DBPath, cfg.Addr(), files[0], time.Now(), force)
	if err != nil {
		return err
	}
	if aside != "" {
		fmt.Println("Bisherige Datenbank gesichert:", aside)
	}
	fmt.Println("Datenbank wiederhergestellt:", cfg.DBPath, "aus", files[0])
	return nil
}

// restoreDB kopiert src nach dbPath. Läuft unter addr ein Server, bricht es ab
// (außer mit force, das nur der bewusste Eingriff von Hand ist). Die bisherige
// Datenbank samt WAL-Resten wandert nach <dbPath>.before-restore-<Zeit>
// (aside, leer wenn es keine gab); es wird nichts gelöscht.
func restoreDB(dbPath, addr, src string, now time.Time, force bool) (aside string, err error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	header := make([]byte, len("SQLite format 3\x00"))
	if _, err := io.ReadFull(in, header); err != nil || string(header) != "SQLite format 3\x00" {
		return "", fmt.Errorf("%s ist keine SQLite-Datenbank", src)
	}
	if !force {
		if ln, err := net.Listen("tcp", addr); err != nil {
			return "", fmt.Errorf("%s ist belegt, läuft Kairo noch? Erst stoppen oder --force", addr)
		} else {
			ln.Close()
		}
	}
	// Erst neben das Ziel kopieren, damit ein Abbruch die bisherige Datenbank nicht berührt.
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dbPath), ".restore-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := in.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil { // CreateTemp legt die Datei schon mit 0600 an
		return "", err
	}
	if _, err := os.Stat(dbPath); err == nil {
		aside = dbPath + ".before-restore-" + now.Format("20060102-150405")
		for _, suffix := range []string{"", "-wal", "-shm"} {
			if err := os.Rename(dbPath+suffix, aside+suffix); err != nil && !(suffix != "" && errors.Is(err, os.ErrNotExist)) {
				return "", err
			}
		}
	}
	return aside, os.Rename(tmp.Name(), dbPath)
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

// importJSON stellt eine Export-Datei (GET /api/export.json) wieder her. Für einen leeren
// Bestand gedacht; läuft der Server noch auf dem konfigurierten Port, bricht es ab.
func importJSON(args []string) error {
	if len(args) != 1 {
		return errors.New("Aufruf: kairo import <datei>")
	}
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	if ln, err := net.Listen("tcp", cfg.Addr()); err != nil {
		return fmt.Errorf("%s ist belegt, läuft Kairo noch? Erst stoppen", cfg.Addr())
	} else {
		ln.Close()
	}
	f, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer f.Close()
	ctx := context.Background()
	db, err := repository.Open(ctx, cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	res, err := importer.Import(ctx, db, f)
	if err != nil {
		return err
	}
	fmt.Printf("Importiert: %d Projekte, %d Aufgaben, %d Gewohnheiten, %d Abhakungen, %d Termine, %d Zeiteinträge, %d Ressourcen\n",
		res.Projects, res.Tasks, res.Habits, res.HabitCompletions, res.CalendarEvents, res.TimeEntries, res.Resources)
	return nil
} // errRestart beendet run, damit dispatch den Prozess neu startet.
var errRestart = errors.New("Neustart")

// reminderStore verbindet TaskRepository und CalendarService für den Erinnerungs-Scheduler.
// journalPath ist die Notiz des Journals für day (wie in frontend/src/lib/journal.ts).
func journalPath(day time.Time) string { return "Journal/" + day.Format("2006-01-02") + ".md" }

type reminderStore struct {
	tasks *repository.TaskRepository
	cal   *service.CalendarService
}

func (s reminderStore) UpcomingTasks(ctx context.Context, lo, hi time.Time) ([]domain.Task, error) {
	return s.tasks.UpcomingTasks(ctx, lo, hi)
}

func (s reminderStore) DueTasks(ctx context.Context, lo, hi time.Time) ([]domain.Task, error) {
	return s.tasks.DueTasks(ctx, lo, hi)
}

func (s reminderStore) Occurrences(ctx context.Context, lo, hi time.Time) ([]domain.EventInstance, error) {
	return s.cal.Occurrences(ctx, lo, hi)
}

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

	var authH *auth.Handler
	var authMW func(http.Handler) http.Handler
	if cfg.RequireAuth() {
		authRepo := auth.NewSQLiteRepository(db)
		authSessions := auth.NewSQLiteSessionStore(db)
		hashConcurrency, err := auth.HashConcurrencyFromEnv()
		if err != nil {
			return err
		}
		hasher, err := auth.NewHasher(hashConcurrency)
		if err != nil {
			return err
		}
		authService := auth.NewServiceWithSessionTTL(authRepo, authSessions, cfg.SessionTTL, hasher)
		authH = auth.NewHandler(authService, cfg.CookieSecure)
		authMW = api.AuthMiddleware(true, token, authSessions, cfg.SessionTTL, cfg.CookieSecure)
	}

	notesStore, err := notes.Open(cfg.NotesDir)
	if err != nil {
		return err
	}
	defer notesStore.Close()

	hub := realtime.NewHub()
	taskRepo := repository.NewTaskRepository(db)
	tasks := service.NewTaskService(taskRepo, nil).WithLocation(cfg.Location)
	journal := service.NewJournalService(notesStore, nil, cfg.Location)
	calendar := service.NewCalendarService(repository.NewCalendarRepository(db), cfg.Location, nil)
	timeEntries := repository.NewTimeEntryRepository(db)
	habitRepo := repository.NewHabitRepository(db)
	habits := service.NewHabitService(habitRepo, cfg.Location, nil).WithTracked(timeEntries)
	timeTracking := service.NewTimeTrackingService(timeEntries, nil).WithLocation(cfg.Location).WithAfterTrack(habits.SyncTracked)
	projects := service.NewProjectService(repository.NewProjectRepository(db), nil)
	goals := service.NewGoalService(repository.NewGoalRepository(db), tasks, timeEntries, habitRepo, cfg.Location, nil)
	for _, p := range []interface{ SetPublisher(service.Publisher) }{projects, tasks, calendar, habits, timeTracking, goals} {
		p.SetPublisher(hub)
	}
	cards := service.NewCardService(notesStore, repository.NewCardRepository(db), cfg.Location, nil)
	review := service.NewReviewService(tasks, projects, habits, calendar, timeTracking, cfg.Location, nil)
	services := api.Services{
		Projects:  projects,
		Tasks:     tasks.WithTimerStopper(timeTracking),
		Journal:   journal,
		Calendar:  calendar,
		Habits:    habits,
		Resources: service.NewResourceService(repository.NewResourceRepository(db), nil),
		Time:      timeTracking,
		Today:     service.NewTodayService(tasks, calendar, habits, timeTracking, cfg.Location, nil).WithWorkWindow(cfg.WorkStart, cfg.WorkEnd),
		Review:    review,
		Goals:     goals,
		Cards:     cards,
		Notes:     notesStore,
		Prefs:     repository.NewPrefsRepository(db),
		Settings:  &api.Settings{Running: cfg, Getenv: os.Getenv, Restart: func() { restart.Store(true); stop() }},
		Hub:       hub,
		Location:  cfg.Location,
		GitHub:    github.New(),
	}

	// Erinnerungen per ntfy: nur wenn URL und Topic gesetzt sind.
	if cfg.NtfyURL != "" && cfg.NtfyTopic != "" {
		rem := reminder.New(reminderStore{taskRepo, calendar}, cfg.NtfyURL, cfg.NtfyTopic, cfg.NtfyLead, 0, cfg.Location, log).
			WithDigest(cfg.NtfyDigest).
			WithDigestLine(func(ctx context.Context) (string, error) {
				g, err := goals.Today(ctx)
				if g == nil || err != nil {
					return "", err
				}
				return g.Line(), nil
			}).
			WithDigestLine(func(ctx context.Context) (string, error) {
				cs, err := cards.Due(ctx)
				if len(cs) == 0 || err != nil {
					return "", err
				}
				return fmt.Sprintf("Lernkarten fällig: %d", len(cs)), nil
			}).
			WithSource(func(ctx context.Context, lo, hi time.Time) ([]reminder.Item, error) {
				rs, err := habits.Reminders(ctx, lo, hi)
				out := make([]reminder.Item, 0, len(rs))
				for _, x := range rs {
					out = append(out, reminder.Item{Key: "habit:" + x.Habit.ID + "@" + x.Date, Title: x.Habit.Name, Detail: "Gewohnheit noch offen", At: x.At})
				}
				return out, err
			}).
			WithJob(time.Sunday, cfg.NtfyWeekly, review.Scorecard).
			WithJob(reminder.Daily, cfg.NtfyJournal, func(_ context.Context, day time.Time) (string, string, error) {
				// Wer heute schon geschrieben hat, wird nicht erinnert.
				if _, _, err := notesStore.Read(journalPath(day)); err == nil {
					return "", "", nil
				}
				return "3 Dinge heute?", "Was lief gut? Was lerne ich daraus? Was ist morgen wichtig?", nil
			})
		log.Info("Erinnerungen aktiv", "ntfy", cfg.NtfyURL+"/"+cfg.NtfyTopic, "lead", cfg.NtfyLead)
		go rem.Run(ctx)
	} else {
		log.Info("Erinnerungen aus (KAIRO_NTFY_URL und KAIRO_NTFY_TOPIC nicht gesetzt)")
	}

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           api.NewRouter(cfg.Port, token, version, web.Handler(), services, authH, authMW, cfg.RequireAuth()),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	log.Info("Kairo gestartet", "url", "http://"+cfg.Host+":"+strconv.Itoa(cfg.Port),
		"version", version, "auth", cfg.RequireAuth(), "db", cfg.DBPath, "notes", cfg.NotesDir, "timezone", cfg.Location.String())

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
