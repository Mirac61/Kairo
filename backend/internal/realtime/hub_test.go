package realtime

import (
	"encoding/json"
	"testing"

	"kairo/internal/domain"
)

func TestHubPublishReachesAllSubscribers(t *testing.T) {
	h := NewHub()
	a, b := h.Subscribe(), h.Subscribe()
	h.Publish(domain.Event{Type: domain.EventTimerStarted, ID: "e1", TaskID: "t1"})
	for _, s := range []*Subscription{a, b} {
		var m map[string]string
		if err := json.Unmarshal(<-s.Messages, &m); err != nil {
			t.Fatal(err)
		}
		if m["type"] != "TIMER_STARTED" || m["id"] != "e1" || m["task_id"] != "t1" {
			t.Errorf("Nachricht = %v", m)
		}
	}
	h.Publish(domain.Event{Type: domain.EventTaskDeleted, ID: "t1"})
	var m map[string]string
	_ = json.Unmarshal(<-a.Messages, &m)
	if _, has := m["task_id"]; has {
		t.Errorf("leeres task_id sollte fehlen: %v", m)
	}
}

func TestHubUnsubscribe(t *testing.T) {
	h := NewHub()
	s := h.Subscribe()
	h.Unsubscribe(s)
	h.Unsubscribe(s) // harmlos
	if _, ok := <-s.Messages; ok || h.Clients() != 0 {
		t.Errorf("Abonnement nicht beendet, Clients = %d", h.Clients())
	}
	h.Publish(domain.Event{Type: domain.EventTaskCreated}) // kein Panic
}

func TestHubDropsSlowSubscriberWithoutBlocking(t *testing.T) {
	h := NewHub()
	slow, fast := h.Subscribe(), h.Subscribe()
	for i := 0; i < subscriberBuffer+5; i++ {
		h.Publish(domain.Event{Type: domain.EventTaskUpdated})
		<-fast.Messages
	}
	if h.Clients() != 1 {
		t.Fatalf("Clients = %d, erwartet 1 (langsamer Client entfernt)", h.Clients())
	}
	n := 0
	for range slow.Messages { // geschlossen: liest Rest und endet
		n++
	}
	if n != subscriberBuffer {
		t.Errorf("langsamer Client bekam %d Nachrichten", n)
	}
	h.Unsubscribe(slow) // schon entfernt, harmlos
}
