package domain

import (
	"crypto/sha1"
	"encoding/hex"
	"math"
	"strings"
	"time"
)

// Card ist eine Karteikarte aus einer Notiz: eine Zeile „Frage :: Antwort“
// (wie im Obsidian-Plugin „Spaced Repetition“).
type Card struct {
	Key      string // Hash der Frage
	ID       string // nur bei Karten direkt in Kairo (ohne Notiz)
	Deck     string // Fach: erster Ordner der Notiz oder frei gewählt
	Question string
	Answer   string
	NotePath string
}

// NoDeck ist das Fach von Karten aus Notizen ohne Ordner.
const NoDeck = "Allgemein"

// DeckOf ist das Fach einer Notiz: ihr erster Ordner.
func DeckOf(notePath string) string {
	if i := strings.Index(notePath, "/"); i > 0 {
		return notePath[:i]
	}
	return NoDeck
}

// CardKey identifiziert eine Karte über ihre Frage; gleiche Fragen teilen sich den Lernstand.
func CardKey(question string) string {
	sum := sha1.Sum([]byte(strings.TrimSpace(question)))
	return hex.EncodeToString(sum[:8])
}

// CardState ist der Lernstand einer Karte (SM-2). Due ist der nächste Tag (YYYY-MM-DD).
type CardState struct {
	Due      string
	Interval int // Tage
	Ease     float64
	Reps     int // richtige Antworten in Folge
}

// CardGrade bewertet eine Antwort: 0 nochmal, 1 schwer, 2 gut, 3 leicht.
type CardGrade int

const (
	GradeAgain CardGrade = iota
	GradeHard
	GradeGood
	GradeEasy
)

// ParseCards liest alle Karten einer Notiz. Zeilen in Codeblöcken zählen nicht.
func ParseCards(notePath, content string) []Card {
	var out []Card
	fence := false
	for _, line := range strings.Split(content, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "```") {
			fence = !fence
			continue
		}
		q, a, ok := strings.Cut(t, " :: ")
		if fence || !ok {
			continue
		}
		q = strings.TrimSpace(strings.TrimLeft(q, "-*+ "))
		a = strings.TrimSpace(a)
		if q == "" || a == "" {
			continue
		}
		out = append(out, Card{Key: CardKey(q), Deck: DeckOf(notePath), Question: q, Answer: a, NotePath: notePath})
	}
	return out
}

// Next berechnet den Lernstand nach einer Antwort am Tag today (SM-2, vereinfacht wie Anki).
// Eine neue Karte hat den Nullwert. „Nochmal“ macht die Karte heute wieder fällig.
func (c CardState) Next(g CardGrade, today time.Time) CardState {
	if c.Ease == 0 {
		c.Ease = 2.5
	}
	switch g {
	case GradeAgain:
		c.Reps, c.Interval, c.Ease = 0, 0, c.Ease-0.2
	case GradeHard:
		c.Interval = max(1, int(math.Round(float64(c.Interval)*1.2)))
		c.Ease -= 0.15
		c.Reps++
	case GradeGood:
		switch c.Reps {
		case 0:
			c.Interval = 1
		case 1:
			c.Interval = 3
		default:
			c.Interval = int(math.Round(float64(c.Interval) * c.Ease))
		}
		c.Reps++
	case GradeEasy:
		if c.Reps == 0 {
			c.Interval = 4
		} else {
			c.Interval = int(math.Round(float64(c.Interval) * c.Ease * 1.3))
		}
		c.Ease += 0.15
		c.Reps++
	}
	c.Ease = max(1.3, c.Ease)
	c.Due = today.AddDate(0, 0, c.Interval).Format("2006-01-02")
	return c
}
