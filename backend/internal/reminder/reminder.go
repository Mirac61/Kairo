// Package reminder erinnert vor dem Beginn von Aufgaben und Terminen per ntfy.
// Der Scheduler läuft im Hintergrund, fragt die fälligen Einträge im Fenster ab
// und verschickt für jeden Eintrag genau eine Benachrichtigung (Vorlauf NtfyLead).
package reminder

import (
	"bytes"
	"context"
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
	}
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
	if !r.Enabled() {
		return nil
	}
	body := "Start um " + at.In(r.loc).Format("15:04")
	if detail != "" {
		body += " · " + detail
	}
	req, err := http.NewRequest(http.MethodPost, r.url+"/"+r.topic, bytes.NewBufferString(body))
	if err != nil {
		return err
	}
	req.Header.Set("Title", title)
	req.Header.Set("Tags", "bell")
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy: %s", resp.Status)
	}
	return nil
}
