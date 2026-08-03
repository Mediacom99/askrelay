package relay

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/Mediacom99/askrelay/internal/envelope"
)

// NOTE: the daemon-signed outbound "submit" path was deferred to WP-09 (WP-08
// quality-pass finding C1: a raw ingest bypassed the outbound approval gate).
// Its adversarial coverage — cross-thread injection, replay, and freshness on a
// signed submit — returns with that path in WP-09; the store-layer equivalents
// remain covered by store/qualitypass_test.go and store/message_test.go.

// ---------------------------------------------------------------------------
// SECURITY (WP-08 exit criterion "revocation severs live sockets", finding H2):
// a device revoked while its socket is open must stop receiving push AND be
// severed — on the push path, on the read (ack) path, and by the reconcile.
// ---------------------------------------------------------------------------

// TestWSRevokedDeviceStopsReceivingPush: a device revoked while connected does
// not receive a re-push of its backlog — pushInbox re-checks ActiveDeviceByID
// and severs instead.
func TestWSRevokedDeviceStopsReceivingPush(t *testing.T) {
	s := testServer(t)
	person, device, cred := enrollDevice(t, s, "marco@example.com")
	sender, _, _ := enrollDevice(t, s, "sender@example.com")
	srv := httptest.NewServer(s.logRequests(s.mux))
	defer srv.Close()

	// Message fanned out to the device while it was still active, delivered on
	// connect but left UNACKED on purpose (the realistic pre-revocation backlog).
	msgID := ingestTo(t, s, sender, person)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := dialWS(ctx, srv.URL, cred)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "bye")
	readMessage(ctx, t, c, msgID)

	if err := s.store.RevokeDevice(device, time.Now().UTC()); err != nil {
		t.Fatalf("RevokeDevice: %v", err)
	}

	// Some later activity triggers a push. The revoked device must not be
	// re-pushed the backlog; its socket is severed instead.
	s.Notify(person)

	readCtx, readCancel := context.WithTimeout(ctx, time.Second)
	defer readCancel()
	_, data, err := c.Read(readCtx)
	if err == nil {
		var f wsFrame
		_ = json.Unmarshal(data, &f)
		if f.Type == "message" {
			t.Errorf("revoked device was re-pushed a message frame (id %q) after revocation", f.ID)
		}
	}
	// And the socket is severed from the hub.
	if !waitFor(func() bool { return s.hub.count(person) == 0 }) {
		t.Error("revoked device's socket was not severed from the hub after a push attempt")
	}
}

// TestWSRevokedDeviceCannotAct: after revocation, the read loop's per-frame
// re-check severs the socket instead of honoring the ack — the message stays
// unacked and the socket leaves the hub.
func TestWSRevokedDeviceCannotAct(t *testing.T) {
	s := testServer(t)
	person, device, cred := enrollDevice(t, s, "marco@example.com")
	sender, _, _ := enrollDevice(t, s, "sender@example.com")
	srv := httptest.NewServer(s.logRequests(s.mux))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	msgID := ingestTo(t, s, sender, person)

	c, _, err := dialWS(ctx, srv.URL, cred)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "bye")
	readMessage(ctx, t, c, msgID)

	if err := s.store.RevokeDevice(device, time.Now().UTC()); err != nil {
		t.Fatalf("RevokeDevice: %v", err)
	}
	ackMessage(ctx, t, c, msgID)

	// Socket severed, and the ack was NOT honored (message still unacked).
	if !waitFor(func() bool { return s.hub.count(person) == 0 }) {
		t.Error("revoked device's socket was not severed on its next frame")
	}
	items, err := s.store.Inbox(device)
	if err != nil {
		t.Fatalf("Inbox: %v", err)
	}
	if len(items) == 0 {
		t.Error("a revoked device's ack was honored — revocation not consulted on the read path")
	}
}

// TestSeverRevoked: the reconcile pass closes an idle socket whose device was
// revoked out of band (the CLI writes the DB directly; the relay never sees it).
func TestSeverRevoked(t *testing.T) {
	s := testServer(t)
	person, device, cred := enrollDevice(t, s, "marco@example.com")
	srv := httptest.NewServer(s.logRequests(s.mux))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := dialWS(ctx, srv.URL, cred)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "bye")
	if !waitFor(func() bool { return s.hub.count(person) == 1 }) {
		t.Fatal("socket not registered")
	}

	if err := s.store.RevokeDevice(device, time.Now().UTC()); err != nil {
		t.Fatalf("RevokeDevice: %v", err)
	}
	s.severRevoked() // what the reconcile ticker runs
	if !waitFor(func() bool { return s.hub.count(person) == 0 }) {
		t.Error("severRevoked did not close the revoked device's idle socket")
	}
}

// ---------------------------------------------------------------------------
// Malformed / oversized / hostile bytes on the wire.
// ---------------------------------------------------------------------------

// TestWSMalformedFrameDoesNotKillConnection: garbage bytes (not JSON, then JSON
// that doesn't match wsFrame, then unknown/unusable frame types) must be logged
// and dropped, never crash the handler or kill the connection.
func TestWSMalformedFrameDoesNotKillConnection(t *testing.T) {
	s := testServer(t)
	person, _, cred := enrollDevice(t, s, "marco@example.com")
	sender, _, _ := enrollDevice(t, s, "sender@example.com")
	srv := httptest.NewServer(s.logRequests(s.mux))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := dialWS(ctx, srv.URL, cred)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "bye")
	if !waitFor(func() bool { return s.hub.count(person) == 1 }) {
		t.Fatal("socket not registered")
	}

	garbage := [][]byte{
		[]byte("not json at all {{{"),
		[]byte(`"just a json string"`),
		[]byte(`42`),
		[]byte(`null`),
		[]byte(`[]`),
		[]byte(`{"type": 123}`),      // type wrong JSON kind
		[]byte(`{"type":"unknown"}`), // unknown frame type → logged, dropped
		[]byte(`{"type":"ack"}`),     // ack with no id → ignored
		{},                           // empty frame
	}
	for _, g := range garbage {
		if err := c.Write(ctx, websocket.MessageText, g); err != nil {
			t.Fatalf("write garbage %q: %v", g, err)
		}
	}

	// The connection must have survived: a real push right after must arrive.
	msgID := ingestTo(t, s, sender, person)
	s.Notify(person)
	readMessage(ctx, t, c, msgID)
}

// TestWSOversizedFrameCloses: a frame over maxWSFrame must be refused by the
// read-limit, ending the connection rather than being buffered/parsed.
func TestWSOversizedFrameCloses(t *testing.T) {
	s := testServer(t)
	_, _, cred := enrollDevice(t, s, "marco@example.com")
	srv := httptest.NewServer(s.logRequests(s.mux))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := dialWS(ctx, srv.URL, cred)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "bye")

	huge := []byte(`{"type":"ack","id":"` + strings.Repeat("A", maxWSFrame+1024) + `"}`)
	_ = c.Write(ctx, websocket.MessageText, huge)

	readCtx, readCancel := context.WithTimeout(ctx, 2*time.Second)
	defer readCancel()
	if _, _, err := c.Read(readCtx); err == nil {
		t.Error("expected the connection to be closed/errored after an oversized frame, read succeeded")
	}
}

// FuzzWSFrameDecode exercises the untrusted-bytes boundary: any bytes a daemon
// could send as a text frame must decode (or fail to decode) without panicking,
// and any embedded envelope bytes fed to envelope.Decode must likewise not panic.
func FuzzWSFrameDecode(f *testing.F) {
	seeds := [][]byte{
		[]byte(`{"type":"ack","id":"x"}`),
		[]byte(`{"type":"unknown","envelope":{}}`),
		[]byte(`{"type":"ack","envelope":{"v":1,"id":"not-a-uuid"}}`),
		[]byte(`{}`),
		[]byte(`null`),
		[]byte(``),
		[]byte(`{"type":"ack","envelope":"` + strings.Repeat("A", 500) + `"}`),
	}
	for _, sd := range seeds {
		f.Add(sd)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic decoding ws frame bytes %q: %v", data, r)
			}
		}()
		var fr wsFrame
		if err := json.Unmarshal(data, &fr); err != nil {
			return
		}
		if len(fr.Envelope) > 0 {
			_, _ = envelope.Decode(fr.Envelope)
		}
	})
}

// ---------------------------------------------------------------------------
// Concurrency on the hub and pushes.
// ---------------------------------------------------------------------------

// TestHubConcurrentReconnectRace hammers add/remove/snapshot for the SAME
// (person, device) key from many goroutines using REAL dialed sockets — run
// with -race. Final occupancy is inherently racy (0 or 1); the point is no data
// race across the add/CloseNow/remove interleavings.
func TestHubConcurrentReconnectRace(t *testing.T) {
	s := testServer(t)
	_, _, cred := enrollDevice(t, s, "marco@example.com")
	srv := httptest.NewServer(s.logRequests(s.mux))
	defer srv.Close()

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			c, _, err := dialWS(ctx, srv.URL, cred)
			if err != nil {
				return // a superseded dial can legitimately race-lose the handshake
			}
			defer c.Close(websocket.StatusNormalClosure, "bye")
			time.Sleep(time.Millisecond)
		}()
	}
	wg.Wait()
}

// TestHubAddNilConnPanics pins the hub.add nil-conn crash directly (finding L5,
// left as-is: unreachable in production, which always constructs wsConn from a
// live websocket.Accept result). Skips if the code is ever hardened.
func TestHubAddNilConnPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Skip("hub.add no longer panics on a nil-conn supersede — nil-safety was added; documentation only")
		}
	}()
	h := newHub()
	h.add("p", "d", &wsConn{conn: nil})
	h.add("p", "d", &wsConn{conn: nil}) // supersede path: old.conn.CloseNow() on a nil conn
	t.Error("expected hub.add to panic on a nil-conn supersede")
}

// TestHubConcurrentMultiDeviceRace stresses many distinct persons/devices
// concurrently, exercising closeAll against live connects/registers — run with -race.
func TestHubConcurrentMultiDeviceRace(t *testing.T) {
	s := testServer(t)
	srv := httptest.NewServer(s.logRequests(s.mux))
	defer srv.Close()

	var wg sync.WaitGroup
	for p := range 8 {
		_, _, cred := enrollDevice(t, s, string(rune('a'+p))+"@example.com")
		for range 4 {
			wg.Add(1)
			go func(cred string) {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				c, _, err := dialWS(ctx, srv.URL, cred)
				if err != nil {
					return
				}
				defer c.Close(websocket.StatusNormalClosure, "bye")
				s.hub.closeAll()
			}(cred)
		}
	}
	wg.Wait()
}

// TestWSConcurrentNotifyRace: many concurrent Notify()+ingest against one live
// socket must not race — the coalescing wake + single writer serialize writes.
func TestWSConcurrentNotifyRace(t *testing.T) {
	s := testServer(t)
	person, _, cred := enrollDevice(t, s, "marco@example.com")
	sender, _, _ := enrollDevice(t, s, "sender@example.com")
	srv := httptest.NewServer(s.logRequests(s.mux))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _, err := dialWS(ctx, srv.URL, cred)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "bye")
	if !waitFor(func() bool { return s.hub.count(person) == 1 }) {
		t.Fatal("socket not registered")
	}

	// Drain frames concurrently so the writer side doesn't block on a full buffer.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := c.Read(ctx); err != nil {
				return
			}
		}
	}()

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			ingestTo(t, s, sender, person)
			s.Notify(person)
		}()
	}
	wg.Wait()
	c.Close(websocket.StatusNormalClosure, "done")
	<-done
}

// dialTwiceConcurrently dials two connections for the SAME device concurrently.
func dialTwiceConcurrently(t *testing.T, base, cred string) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var c1, c2 *websocket.Conn
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); c1, _, _ = dialWS(ctx, base, cred) }()
	go func() { defer wg.Done(); c2, _, _ = dialWS(ctx, base, cred) }()
	wg.Wait()
	return c1, c2
}

func TestWSDoubleConnectSameDeviceRace(t *testing.T) {
	s := testServer(t)
	person, _, cred := enrollDevice(t, s, "marco@example.com")
	srv := httptest.NewServer(s.logRequests(s.mux))
	defer srv.Close()

	c1, c2 := dialTwiceConcurrently(t, srv.URL, cred)
	if c1 == nil || c2 == nil {
		t.Fatal("both concurrent dials must succeed at the HTTP/WS layer")
	}
	defer c1.Close(websocket.StatusNormalClosure, "bye")
	defer c2.Close(websocket.StatusNormalClosure, "bye")

	// Exactly one registration must remain for the device — never two, never zero.
	if !waitFor(func() bool { return s.hub.count(person) == 1 }) {
		t.Errorf("hub count after double-connect race = %d, want 1", s.hub.count(person))
	}
}

// ---------------------------------------------------------------------------
// ack/delivery isolation across devices and persons.
// ---------------------------------------------------------------------------

// TestWSAckCrossDeviceIsolation: a device cannot ack a message only fanned out
// to a DIFFERENT device — requireDelivery scopes on (message, THIS device).
func TestWSAckCrossDeviceIsolation(t *testing.T) {
	s := testServer(t)
	person, deviceA, credA := enrollDevice(t, s, "marco@example.com")
	_, deviceB, _ := enrollDevice(t, s, "marco@example.com") // same person, 2nd device
	sender, _, _ := enrollDevice(t, s, "sender@example.com")
	srv := httptest.NewServer(s.logRequests(s.mux))
	defer srv.Close()

	msgID := ingestTo(t, s, sender, person) // fans out to BOTH marco devices

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cA, _, err := dialWS(ctx, srv.URL, credA)
	if err != nil {
		t.Fatalf("dial A: %v", err)
	}
	defer cA.Close(websocket.StatusNormalClosure, "bye")
	readMessage(ctx, t, cA, msgID)
	ackMessage(ctx, t, cA, msgID)

	if !waitFor(func() bool {
		items, e := s.store.Inbox(deviceA)
		return e == nil && len(items) == 0
	}) {
		t.Fatalf("deviceA inbox not cleared by its own ack")
	}
	items, err := s.store.Inbox(deviceB)
	if err != nil {
		t.Fatalf("Inbox(deviceB): %v", err)
	}
	found := false
	for _, it := range items {
		if it.ID == msgID {
			found = true
		}
	}
	if !found {
		t.Error("device A's ack cleared device B's independent delivery row for the same message")
	}
}

// TestWSAckUnrelatedMessageIgnored: acking a message id never delivered to this
// device (someone else's mail, or garbage) is a no-op, never touching other rows.
func TestWSAckUnrelatedMessageIgnored(t *testing.T) {
	s := testServer(t)
	victim, victimDevice, _ := enrollDevice(t, s, "victim@example.com")
	_, _, credAtk := enrollDevice(t, s, "eve@example.com")
	sender, _, _ := enrollDevice(t, s, "sender@example.com")
	srv := httptest.NewServer(s.logRequests(s.mux))
	defer srv.Close()

	victimMsg := ingestTo(t, s, sender, victim)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cAtk, _, err := dialWS(ctx, srv.URL, credAtk)
	if err != nil {
		t.Fatalf("dial attacker: %v", err)
	}
	defer cAtk.Close(websocket.StatusNormalClosure, "bye")

	ackMessage(ctx, t, cAtk, victimMsg)    // someone else's message id
	ackMessage(ctx, t, cAtk, "not-a-uuid") // garbage id

	time.Sleep(100 * time.Millisecond)
	items, err := s.store.Inbox(victimDevice)
	if err != nil {
		t.Fatalf("Inbox(victim): %v", err)
	}
	found := false
	for _, it := range items {
		if it.ID == victimMsg {
			found = true
		}
	}
	if !found {
		t.Error("an unrelated device's ack for someone else's message id cleared that message's delivery")
	}
}
