package daemon

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// testDaemon builds a Daemon pointed at relayURL whose notifications land in
// notes instead of on a real desktop.
func testDaemon(relayURL string, notes chan<- struct{}) *Daemon {
	return &Daemon{
		cfg:    Config{RelayURL: relayURL, PersonID: "p1", DeviceID: "d1", DeviceCredential: "cred"},
		log:    slog.New(slog.DiscardHandler),
		seen:   map[string]bool{},
		notify: func() { notes <- struct{}{} },
	}
}

// The two invariants ST-2 rests on: one notification per distinct message id
// (the relay re-pushes unacked mail on every wake), and not a single frame sent
// back — an ack would arm the relay's ack-grace deletion timer against a
// message its human has not read.
func TestSessionDedupesAndNeverAcks(t *testing.T) {
	notes := make(chan struct{}, 8)
	sent := make(chan string, 4) // anything the daemon writes to us

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer cred" {
			t.Errorf("Authorization = %q, want the device credential", got)
		}
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		defer func() { _ = c.CloseNow() }()
		for _, f := range []frame{
			{Type: "message", ID: "m1", ThreadID: "t1"},
			{Type: "message", ID: "m1", ThreadID: "t1"}, // re-push, as the relay does
			{Type: "message", ID: "m2", ThreadID: "t1"},
			{Type: "surprise"}, // unknown type: ignored, not fatal
		} {
			b, _ := json.Marshal(f)
			if err := c.Write(r.Context(), websocket.MessageText, b); err != nil {
				return
			}
		}
		ctx, cancel := context.WithTimeout(r.Context(), 300*time.Millisecond)
		defer cancel()
		if _, data, err := c.Read(ctx); err == nil {
			sent <- string(data)
		}
	}))
	defer srv.Close()

	d := testDaemon(srv.URL, notes)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- d.session(ctx) }()

	for i := range 2 {
		select {
		case <-notes:
		case <-time.After(2 * time.Second):
			t.Fatalf("notification %d never arrived", i+1)
		}
	}
	select {
	case f := <-sent:
		t.Fatalf("daemon sent a frame; it must never ack: %s", f)
	case <-time.After(500 * time.Millisecond):
	}
	select {
	case <-notes:
		t.Fatal("a duplicate message id was notified twice")
	default:
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("session did not return after cancellation")
	}
}

// A refused credential (revoked device, stale credential) is terminal: retrying
// it forever would just be noise on a 30 s timer.
func TestListenStopsOnUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := testDaemon(srv.URL, make(chan struct{}, 1))
	done := make(chan struct{})
	go func() { d.listen(ctx); close(done) }()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("listen kept retrying a rejected credential")
	}
}
