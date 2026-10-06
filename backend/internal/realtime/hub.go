// Package realtime verteilt Ereignisse an WebSocket-Clients.
package realtime

import (
	"encoding/json"
	"log/slog"
	"sync"

	"kairo/internal/domain"
)

// subscriberBuffer ist die Zahl der Nachrichten, die ein Client im Rückstand sein darf.
const subscriberBuffer = 64

// Hub verteilt Ereignisse an alle Abonnenten. Publish blockiert nie: Wer
// zu langsam ist, wird entfernt und muss sich neu verbinden (und seinen
// Stand per REST nachladen).
type Hub struct {
	mu   sync.Mutex
	subs map[*Subscription]struct{}
}

// Subscription ist ein Abonnement. Messages wird geschlossen, wenn es
// endet (Unsubscribe oder zu langsam).
type Subscription struct {
	Messages <-chan []byte
	ch       chan []byte
}

// NewHub erzeugt einen leeren Hub.
func NewHub() *Hub { return &Hub{subs: map[*Subscription]struct{}{}} }

type message struct {
	Type   domain.EventType `json:"type"`
	ID     string           `json:"id,omitempty"`
	TaskID string           `json:"task_id,omitempty"`
}

// Publish sendet e an alle Abonnenten.
func (h *Hub) Publish(e domain.Event) {
	data, err := json.Marshal(message{Type: e.Type, ID: e.ID, TaskID: e.TaskID})
	if err != nil {
		slog.Error("Ereignis kodieren", "err", err)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for s := range h.subs {
		select {
		case s.ch <- data:
		default:
			slog.Warn("WebSocket-Client zu langsam, getrennt")
			delete(h.subs, s)
			close(s.ch)
		}
	}
}

// Subscribe meldet einen neuen Abonnenten an.
func (h *Hub) Subscribe() *Subscription {
	ch := make(chan []byte, subscriberBuffer)
	s := &Subscription{Messages: ch, ch: ch}
	h.mu.Lock()
	h.subs[s] = struct{}{}
	h.mu.Unlock()
	return s
}

// Unsubscribe beendet ein Abonnement. Mehrfaches Aufrufen ist harmlos.
func (h *Hub) Unsubscribe(s *Subscription) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.subs[s]; ok {
		delete(h.subs, s)
		close(s.ch)
	}
}

// Clients liefert die Zahl der aktuellen Abonnenten.
func (h *Hub) Clients() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.subs)
}
