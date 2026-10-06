package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"kairo/internal/realtime"
	"kairo/internal/repository"
	"kairo/internal/service"
)

// newWSServer startet einen echten HTTP-Server. Security prüft den Host
// gegen Port 8742, deshalb setzt dialWS den Host-Header selbst.
func newWSServer(t *testing.T) (*httptest.Server, http.Handler) {
	t.Helper()
	db, err := repository.Open(context.Background(), filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	hub := realtime.NewHub()
	timeSvc := service.NewTimeTrackingService(repository.NewTimeEntryRepository(db), nil)
	tasks := service.NewTaskService(repository.NewTaskRepository(db), nil)
	projects := service.NewProjectService(repository.NewProjectRepository(db), nil)
	for _, p := range []interface{ SetPublisher(service.Publisher) }{timeSvc, tasks, projects} {
		p.SetPublisher(hub)
	}
	h := NewRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{
		Projects: projects, Tasks: tasks.WithTimerStopper(timeSvc), Time: timeSvc, Hub: hub,
	})
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, h
}

func dialWS(t *testing.T, srv *httptest.Server, hdr http.Header) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	hdr.Set("Host", "127.0.0.1:8742")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	return websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", &websocket.DialOptions{HTTPHeader: hdr, Host: "127.0.0.1:8742"})
}

func readEvent(t *testing.T, conn *websocket.Conn) map[string]string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("WebSocket lesen: %v", err)
	}
	var m map[string]string
	_ = json.Unmarshal(data, &m)
	return m
}

func TestWebSocketDeliversEvents(t *testing.T) {
	srv, h := newWSServer(t)
	conn, _, err := dialWS(t, srv, http.Header{"Authorization": {"Bearer " + testToken}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()

	var task taskDTO
	_ = json.Unmarshal(call(h, "POST", "/api/tasks", `{"title":"A"}`).Body.Bytes(), &task)
	if m := readEvent(t, conn); m["type"] != "TASK_CREATED" || m["id"] != task.ID {
		t.Errorf("Ereignis = %v", m)
	}

	var res timerDTO
	_ = json.Unmarshal(call(h, "POST", "/api/tasks/"+task.ID+"/start", "").Body.Bytes(), &res)
	if m := readEvent(t, conn); m["type"] != "TASK_STARTED" || m["id"] != task.ID {
		t.Errorf("Ereignis = %v", m)
	}
	if m := readEvent(t, conn); m["type"] != "TIMER_STARTED" || m["id"] != res.TimeEntry.ID || m["task_id"] != task.ID {
		t.Errorf("Ereignis = %v", m)
	}

	// Ein zweiter Start derselben Task ändert nichts und meldet nichts.
	call(h, "POST", "/api/tasks/"+task.ID+"/start", "")
	call(h, "POST", "/api/tasks/"+task.ID+"/pause", "")
	if m := readEvent(t, conn); m["type"] != "TIMER_STOPPED" || m["task_id"] != task.ID {
		t.Errorf("nach Pause: %v", m)
	}
	if m := readEvent(t, conn); m["type"] != "TASK_PAUSED" {
		t.Errorf("nach Pause: %v", m)
	}
}

func TestWebSocketRejectsWithoutCredentials(t *testing.T) {
	srv, _ := newWSServer(t)
	if _, resp, err := dialWS(t, srv, http.Header{}); err == nil || resp == nil || resp.StatusCode != 403 {
		t.Errorf("ohne Token/Origin: err %v, resp %v", err, resp)
	}
	if _, resp, err := dialWS(t, srv, http.Header{"Origin": {"https://evil.example"}}); err == nil || resp == nil || resp.StatusCode != 403 {
		t.Errorf("fremder Origin: err %v, resp %v", err, resp)
	}
	if _, resp, err := dialWS(t, srv, http.Header{"Authorization": {"Bearer falsch"}}); err == nil || resp == nil || resp.StatusCode != 401 {
		t.Errorf("falsches Token: err %v, resp %v", err, resp)
	}
	conn, _, err := dialWS(t, srv, http.Header{"Origin": {"http://127.0.0.1:8742"}})
	if err != nil {
		t.Fatalf("eigener Origin: %v", err)
	}
	conn.CloseNow()
}
