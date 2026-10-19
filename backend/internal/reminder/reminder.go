// Package reminder erinnert vor dem Beginn von Aufgaben und Terminen per ntfy.
// Der Scheduler läuft im Hintergrund, fragt die fälligen Einträge im Fenster ab
// und verschickt für jeden Eintrag genau eine Benachrichtigung (Vorlauf NtfyLead).
// Fälligkeiten haben keine Uhrzeit (gespeichert als 23:59), dafür gibt es morgens
// eine Sammelmeldung „heute fällig“ mit dem Überfälligen.
package reminder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"kairo/internal/domain"
)

// Store liefert die Einträge, die im Fenster (lo, hi] beginnen.
type Store interface {
	UpcomingTasks(ctx context.Context, lo, hi time.Time) ([]domain.Task, error)
	DueTasks(ctx context.Context, lo, hi time.Time) ([]domain.Task, error)
	Occurrences(ctx context.Context, lo, hi time.Time) ([]domain.EventInstance, error)
}

// Runner erinnert per ntfy. url und topic sind die ntfy-Adresse (leer = nichts senden).
type Runner struct {
	store    Store
	url      string
	topic    string
	lead     time.Duration
	interval time.Duration
	loc      *time.Location
	now      func() time.Time
	log      *slog.Logger
	client   *http.Client
	sent     map[string]time.Time
	digest   time.Duration // Uhrzeit der Sammelmeldung ab Mitternacht; < 0 = aus
	digested string        // Tag (YYYY-MM-DD) der letzten Sammelmeldung
}

// New erzeugt einen Runner. interval ist der Abstand zwischen zwei Abfragen (≤ 0 = eine Minute).
func New(store Store, url, topic string, lead, interval time.Duration, loc *time.Location, log *slog.Logger) *Runner {
	if interval <= 0 {
		interval = time.Minute
	}
	if loc == nil {
		loc = time.Local
	}
	if log == nil {
		log = slog.Default()
	}
	return &Runner{
		store:    store,
		url:      strings.TrimRight(url, "/"),
		topic:    topic,
		lead:     lead,
		interval: interval,
		loc:      loc,
		now:      time.Now,
		log:      log,
		client:   &http.Client{Timeout: 10 * time.Second},
		sent:     map[string]time.Time{},
		digest:   -1,
	}
}

// WithDigest schaltet die tägliche Meldung „heute fällig“ um at nach Mitternacht (Ortszeit) ein; < 0 = aus.
func (r *Runner) WithDigest(at time.Duration) *Runner {
	r.digest = at
	return r
}

// Enabled sagt, ob Erinnerungen konfiguriert sind.
func (r *Runner) Enabled() bool { return r.url != "" && r.topic != "" }

// Run tickt bis ctx endet.
func (r *Runner) Run(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.tick(ctx)
		}
	}
}

type item struct {
	key, title, detail string
	at                 time.Time
}

func (r *Runner) tick(ctx context.Context) {
	now := r.now().UTC()
	hi := now.Add(r.lead)
	lo := now.Add(-r.interval) // ein Tick Rückstand, damit ein verspäteter Tick nichts verpasst

	var items []item
	if ts, err := r.store.UpcomingTasks(ctx, lo, hi); err != nil {
		r.log.Warn("Erinnerung: Aufgaben lesen", "err", err)
	} else {
		for _, t := range ts {
			if t.PlannedStartAt == nil {
				continue
			}
			items = append(items, item{key: "task:" + t.ID, title: t.Title, at: *t.PlannedStartAt})
		}
	}
	if es, err := r.store.Occurrences(ctx, lo, hi); err != nil {
		r.log.Warn("Erinnerung: Termine lesen", "err", err)
	} else {
		for _, e := range es {
			items = append(items, item{key: "event:" + e.Event.ID, title: e.Event.Title, detail: e.Event.Location, at: e.Start})
		}
	}

	r.dueDigest(ctx, now)
	r.prune(now)
	for _, it := range items {
		if _, seen := r.sent[it.key]; seen {
			continue
		}
		if err := r.send(it.title, it.at, it.detail); err != nil {
			r.log.Warn("Erinnerung senden", "err", err)
			continue
		}
		r.sent[it.key] = now
	}
}

// dueDigest schickt einmal am Tag, in der Stunde ab r.digest, die offenen Fälligkeiten von heute und das Überfällige.
// Startet der Server später am Tag, entfällt die Meldung für diesen Tag (kein Nachholen um Mitternacht).
func (r *Runner) dueDigest(ctx context.Context, now time.Time) {
	if r.digest < 0 {
		return
	}
	local := now.In(r.loc)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, r.loc)
	at := day.Add(r.digest)
	key := day.Format("2006-01-02")
	if r.digested == key || local.Before(at) || !local.Before(at.Add(time.Hour)) {
		return
	}
	ts, err := r.store.DueTasks(ctx, time.Time{}, day.AddDate(0, 0, 1))
	if err != nil {
		r.log.Warn("Erinnerung: Fälliges lesen", "err", err)
		return
	}
	if len(ts) == 0 {
		r.digested = key
		return
	}
	var today, late []string
	for _, t := range ts {
		if t.DueAt.Before(day) {
			late = append(late, t.Title)
		} else {
			today = append(today, t.Title)
		}
	}
	title := fmt.Sprintf("Heute fällig: %d", len(today))
	body := strings.Join(today, "\n")
	if len(late) > 0 {
		title += fmt.Sprintf(" · überfällig: %d", len(late))
		if body != "" {
			body += "\n"
		}
		body += "Überfällig: " + strings.Join(late, ", ")
	}
	if err := r.post(title, body); err != nil {
		r.log.Warn("Erinnerung senden", "err", err)
		return
	}
	r.digested = key
}

// prune entfernt Einträge, deren Fenster längst vorbei ist, damit die Map klein bleibt.
func (r *Runner) prune(now time.Time) {
	cutoff := now.Add(-r.lead - 2*r.interval)
	for k, at := range r.sent {
		if at.Before(cutoff) {
			delete(r.sent, k)
		}
	}
}

// send schickt eine ntfy-Benachrichtigung; ohne url/topic passiert nichts.
func (r *Runner) send(title string, at time.Time, detail string) error {
	body := "Start um " + at.In(r.loc).Format("15:04")
	if detail != "" {
		body += " · " + detail
	}
	return r.post(title, body)
}

// post veröffentlicht als JSON an die Wurzel-URL: Titel mit Umlauten kämen als Header nicht sicher an.
func (r *Runner) post(title, body string) error {
	if !r.Enabled() {
		return nil
	}
	msg, err := json.Marshal(map[string]any{"topic": r.topic, "title": title, "message": body, "tags": []string{"bell"}})
	if err != nil {
		return err
	}
	resp, err := r.client.Post(r.url, "application/json", bytes.NewReader(msg))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy: %s", resp.Status)
	}
	return nil
}
