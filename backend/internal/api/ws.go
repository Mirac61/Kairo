package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"kairo/internal/realtime"
)

const (
	wsPingInterval = 30 * time.Second
	wsWriteTimeout = 5 * time.Second
)

// handleWebSocket schickt jedes Ereignis des Hubs als Textnachricht
// {"type":…,"id":…,"task_id":…}. Origin und Token hat schon Security geprüft,
// deshalb schaltet Accept seine eigene Origin-Prüfung ab. Clients senden
// nichts; eingehende Nachrichten beenden die Verbindung.
func handleWebSocket(hub *realtime.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return // Accept hat die Antwort schon geschrieben
		}
		defer conn.CloseNow()

		sub := hub.Subscribe()
		defer hub.Unsubscribe(sub)
		ctx := conn.CloseRead(r.Context())

		ping := time.NewTicker(wsPingInterval)
		defer ping.Stop()
		for {
			select {
			case msg, ok := <-sub.Messages:
				if !ok {
					conn.Close(websocket.StatusPolicyViolation, "zu langsam")
					return
				}
				if err := writeWS(ctx, conn, msg); err != nil {
					return
				}
			case <-ping.C:
				pctx, cancel := context.WithTimeout(ctx, wsWriteTimeout)
				err := conn.Ping(pctx)
				cancel()
				if err != nil {
					slog.Debug("WebSocket-Ping fehlgeschlagen", "err", err)
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}
}

func writeWS(ctx context.Context, conn *websocket.Conn, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, wsWriteTimeout)
	defer cancel()
	return conn.Write(ctx, websocket.MessageText, msg)
}
