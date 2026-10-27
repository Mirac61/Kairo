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
	jobs     []*job
	extras   []func(ctx context.Context) (string, error) // weitere Zeilen der Sammelmeldung
	sources  []Source
}

// Item ist eine Erinnerung aus einer weiteren Quelle; Key macht sie eindeutig (eine Meldung pro Key).
type Item struct {
	Key, Title, Detail string
	At                 time.Time
}

// Source liefert weitere Erinnerungen, die im Fenster (lo, hi] beginnen (z. B. offene Gewohnheiten).
type Source func(ctx context.Context, lo, hi time.Time) ([]Item, error)

// Daily ist der Wochentag eines Jobs, der jeden Tag läuft.
const Daily time.Weekday = -1

// job ist eine geplante Meldung: einmal am Tag (oder nur an weekday), in der Stunde ab at.
// build liefert Titel und Text; ein leerer Titel heißt „heute nichts zu melden“.
// Startet der Server später am Tag, entfällt die Meldung für diesen Tag (kein Nachholen).
type job struct {
	weekday time.Weekday
	at      time.Duration // ab Mitternacht, Ortszeit
	build   func(ctx context.Context, day time.Time) (title, body string, err error)
	last    string // Tag (YYYY-MM-DD) der letzten Meldung
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

// WithDigest schaltet die tägliche Meldung „heute fällig“ um at nach Mitternacht (Ortszeit) ein; < 0 = aus.
func (r *Runner) WithDigest(at time.Duration) *Runner {
	return r.WithJob(Daily, at, r.digest)
}

// WithDigestLine hängt an die Sammelmeldung eine Zeile an (z. B. das Ziel des Tages); "" = keine.
func (r *Runner) WithDigestLine(line func(ctx context.Context) (string, error)) *Runner {
	r.extras = append(r.extras, line)
	return r
}

// WithJob plant eine Meldung um at nach Mitternacht (Ortszeit), täglich (Daily) oder an weekday; at < 0 = aus.
func (r *Runner) WithJob(weekday time.Weekday, at time.Duration, build func(ctx context.Context, day time.Time) (title, body string, err error)) *Runner {
	if at >= 0 {
		r.jobs = append(r.jobs, &job{weekday: weekday, at: at, build: build})
	}
	return r
}

// WithSource nimmt eine weitere Quelle für Erinnerungen dazu.
func (r *Runner) WithSource(src Source) *Runner {
	r.sources = append(r.sources, src)
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
			// Occurrences liefert auch laufende Termine; die würden sonst nach prune erneut gemeldet.
			if e.Start.Before(lo) {
				continue
			}
			key := "event:" + e.Event.ID + "@" + e.Start.UTC().Format(time.RFC3339)
			items = append(items, item{key: key, title: e.Event.Title, detail: e.Event.Location, at: e.Start})
		}
	}
	for _, src := range r.sources {
		xs, err := src(ctx, lo, hi)
		if err != nil {
			r.log.Warn("Erinnerung: Quelle lesen", "err", err)
			continue
		}
		for _, x := range xs {
			items = append(items, item{key: x.Key, title: x.Title, detail: x.Detail, at: x.At})
		}
	}

	for _, j := range r.jobs {
		r.runJob(ctx, j, now)
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

func (r *Runner) runJob(ctx context.Context, j *job, now time.Time) {
	local := now.In(r.loc)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, r.loc)
	at := day.Add(j.at)
	key := day.Format("2006-01-02")
	if j.last == key || (j.weekday != Daily && local.Weekday() != j.weekday) || local.Before(at) || !local.Before(at.Add(time.Hour)) {
		return
	}
	title, body, err := j.build(ctx, day)
	if err != nil {
		r.log.Warn("Erinnerung: Meldung bauen", "err", err)
		return
	}
	if title != "" {
		if err := r.post(title, body); err != nil {
			r.log.Warn("Erinnerung senden", "err", err)
			return
		}
	}
	j.last = key
}

// digest: die offenen Fälligkeiten von heute, das Überfällige und die Zusatzzeilen (WithDigestLine).
func (r *Runner) digest(ctx context.Context, day time.Time) (string, string, error) {
	ts, err := r.store.DueTasks(ctx, time.Time{}, day.AddDate(0, 0, 1))
	if err != nil {
		return "", "", fmt.Errorf("Fälliges lesen: %w", err)
	}
	var today, late, lines []string
	for _, t := range ts {
		if t.DueAt.Before(day) {
			late = append(late, t.Title)
		} else {
			today = append(today, t.Title)
		}
	}
	for _, extra := range r.extras {
		line, err := extra(ctx)
		if err != nil {
			r.log.Warn("Erinnerung: Zusatzzeile", "err", err)
		} else if line != "" {
			lines = append(lines, line)
		}
	}
	if len(ts) == 0 {
		if len(lines) == 0 {
			return "", "", nil
		}
		return "Heute nichts fällig", strings.Join(lines, "\n"), nil
	}
	title := fmt.Sprintf("Heute fällig: %d", len(today))
	body := today
	if len(late) > 0 {
		title += fmt.Sprintf(" · überfällig: %d", len(late))
		body = append(body, "Überfällig: "+strings.Join(late, ", "))
	}
	return title, strings.Join(append(body, lines...), "\n"), nil
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
