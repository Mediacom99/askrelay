package relay

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/Mediacom99/askrelay/internal/a2a"
	"github.com/Mediacom99/askrelay/internal/envelope"
	"github.com/Mediacom99/askrelay/internal/relay/store"
)

// ingestTo ingests a fresh message from sender to recipient (a new thread),
// fanning out a delivery row to the recipient's active devices. Returns the
// message id.
func ingestTo(t *testing.T, s *Server, sender, recipient string) string {
	t.Helper()
	now := time.Now().UTC()
	e := envelope.New(envelope.Party{Person: sender}, recipient, "", a2a.StateSubmitted, false,
		envelope.Message{Role: "user", Parts: []envelope.Part{{Type: "text", Text: "hi"}}})
	fresh := store.Freshness{MaxAge: 24 * time.Hour, MaxSkew: time.Minute}
	if err := s.store.IngestMessage(e, sender, fresh, now); err != nil {
		t.Fatalf("IngestMessage: %v", err)
	}
	return e.ID
}

// readMessage reads one text frame and asserts it is a "message" for wantID.
func readMessage(ctx context.Context, t *testing.T, c *websocket.Conn, wantID string) {
	t.Helper()
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	var f wsFrame
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("unmarshal frame: %v", err)
	}
	if f.Type != "message" || f.ID != wantID {
		t.Fatalf("frame = %+v, want type=message id=%q", f, wantID)
	}
}

func ackMessage(ctx context.Context, t *testing.T, c *websocket.Conn, id string) {
	t.Helper()
	b, _ := json.Marshal(wsFrame{Type: "ack", ID: id})
	if err := c.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatalf("write ack: %v", err)
	}
}

// enrollDevice registers a fresh device and returns its ids + WS credential.
func enrollDevice(t *testing.T, s *Server, email string) (personID, deviceID, cred string) {
	t.Helper()
	now := time.Now().UTC()
	tok, err := s.store.CreateInvite(email, time.Hour, now)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	person, device, err := s.store.Enroll(tok, pub, "test", now)
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	cred, err = s.issuer.MintDeviceCredential(person.ID, device.ID, now)
	if err != nil {
		t.Fatalf("MintDeviceCredential: %v", err)
	}
	return person.ID, device.ID, cred
}

func wsURL(base string) string { return "ws" + strings.TrimPrefix(base, "http") + "/ws" }

func dialWS(ctx context.Context, base, cred string) (*websocket.Conn, *http.Response, error) {
	opts := &websocket.DialOptions{}
	if cred != "" {
		opts.HTTPHeader = http.Header{"Authorization": {"Bearer " + cred}}
	}
	return websocket.Dial(ctx, wsURL(base), opts)
}

// count reports how many live sockets the hub holds for person.
func (h *hub) count(person string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.conns[person])
}

// waitFor polls cond up to ~2s; registration/teardown run server-side and are
// observed asynchronously by the test.
func waitFor(cond func() bool) bool {
	for range 200 {
		if cond() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return cond()
}

func TestWSConnectAndRegister(t *testing.T) {
	s := testServer(t)
	person, _, cred := enrollDevice(t, s, "marco@example.com")
	srv := httptest.NewServer(s.logRequests(s.mux))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	c, _, err := dialWS(ctx, srv.URL, cred)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if !waitFor(func() bool { return s.hub.count(person) == 1 }) {
		t.Fatalf("hub count = %d, want 1 after connect", s.hub.count(person))
	}

	c.Close(websocket.StatusNormalClosure, "bye")
	if !waitFor(func() bool { return s.hub.count(person) == 0 }) {
		t.Fatalf("hub count = %d, want 0 after close", s.hub.count(person))
	}
}

func TestWSDeliverOnConnectAndAck(t *testing.T) {
	s := testServer(t)
	person, device, cred := enrollDevice(t, s, "marco@example.com")
	sender, _, _ := enrollDevice(t, s, "sender@example.com")
	srv := httptest.NewServer(s.logRequests(s.mux))
	defer srv.Close()

	// Message waiting before the daemon connects → resent on connect.
	msgID := ingestTo(t, s, sender, person)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := dialWS(ctx, srv.URL, cred)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "bye")

	readMessage(ctx, t, c, msgID)
	ackMessage(ctx, t, c, msgID)

	// The ack clears the device's unacked inbox (the retention signal).
	if !waitFor(func() bool {
		items, e := s.store.Inbox(device)
		return e == nil && len(items) == 0
	}) {
		t.Fatalf("inbox not empty after ack")
	}
}

func TestWSLivePush(t *testing.T) {
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
	// Ensure the socket is registered before the delivery so Notify sees it.
	if !waitFor(func() bool { return s.hub.count(person) == 1 }) {
		t.Fatal("socket not registered")
	}

	// Deliver after connect, then push — the frame arrives without a reconnect.
	msgID := ingestTo(t, s, sender, person)
	s.Notify(person)
	readMessage(ctx, t, c, msgID)
}

// enrollSigningDevice is enrollDevice that also returns the device private key,
// so the test can sign envelopes as that device.
func enrollSigningDevice(t *testing.T, s *Server, email string) (personID, deviceID, cred string, priv ed25519.PrivateKey) {
	t.Helper()
	now := time.Now().UTC()
	tok, err := s.store.CreateInvite(email, time.Hour, now)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	pub, p, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	person, device, err := s.store.Enroll(tok, pub, "test", now)
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	cred, err = s.issuer.MintDeviceCredential(person.ID, device.ID, now)
	if err != nil {
		t.Fatalf("MintDeviceCredential: %v", err)
	}
	return person.ID, device.ID, cred, p
}

func signedEnvelope(t *testing.T, from, to, thread string, priv ed25519.PrivateKey) envelope.Envelope {
	t.Helper()
	e := envelope.New(envelope.Party{Person: from}, to, thread, a2a.StateSubmitted, false,
		envelope.Message{Role: "user", Parts: []envelope.Part{{Type: "text", Text: "hi"}}})
	if err := envelope.Sign(&e, priv); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	return e
}

func submitEnvelope(ctx context.Context, t *testing.T, c *websocket.Conn, e envelope.Envelope) {
	t.Helper()
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}
	b, _ := json.Marshal(wsFrame{Type: "submit", Envelope: raw})
	if err := c.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatalf("write submit: %v", err)
	}
}

// inboundIDs is the set of message ids currently awaiting a person.
func inboundIDs(t *testing.T, s *Server, person string) map[string]bool {
	t.Helper()
	items, err := s.store.InboundAwaiting(person)
	if err != nil {
		t.Fatalf("InboundAwaiting: %v", err)
	}
	ids := map[string]bool{}
	for _, it := range items {
		ids[it.MessageID] = true
	}
	return ids
}

func TestWSSubmit(t *testing.T) {
	s := testServer(t)
	sender, _, cred, priv := enrollSigningDevice(t, s, "sender@example.com")
	recipient, _, _ := enrollDevice(t, s, "marco@example.com")
	srv := httptest.NewServer(s.logRequests(s.mux))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := dialWS(ctx, srv.URL, cred)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "bye")

	e := signedEnvelope(t, sender, recipient, "", priv)
	submitEnvelope(ctx, t, c, e)
	if !waitFor(func() bool { return inboundIDs(t, s, recipient)[e.ID] }) {
		t.Fatalf("submitted message not ingested")
	}
}

func TestWSSubmitRejected(t *testing.T) {
	s := testServer(t)
	sender, _, cred, priv := enrollSigningDevice(t, s, "sender@example.com")
	recipient, _, _ := enrollDevice(t, s, "marco@example.com")
	other, _, _ := enrollDevice(t, s, "eve@example.com")
	srv := httptest.NewServer(s.logRequests(s.mux))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := dialWS(ctx, srv.URL, cred)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close(websocket.StatusNormalClosure, "bye")

	// (a) signed by a key that is not this device's → Verify fails.
	_, wrongPriv, _ := ed25519.GenerateKey(rand.Reader)
	bad1 := signedEnvelope(t, sender, recipient, "", wrongPriv)
	submitEnvelope(ctx, t, c, bad1)

	// (b) correctly signed by this device but claims a different From.Person →
	// the binding check rejects it.
	bad2 := signedEnvelope(t, other, recipient, "", priv)
	submitEnvelope(ctx, t, c, bad2)

	// Sentinel: a valid submit that MUST land, proving the loop advanced past
	// both bad frames (each on its own thread, so a leak would show up).
	good := signedEnvelope(t, sender, recipient, "", priv)
	submitEnvelope(ctx, t, c, good)
	if !waitFor(func() bool { return inboundIDs(t, s, recipient)[good.ID] }) {
		t.Fatalf("sentinel not ingested")
	}
	ids := inboundIDs(t, s, recipient)
	if ids[bad1.ID] {
		t.Error("wrong-key signature was ingested")
	}
	if ids[bad2.ID] {
		t.Error("From.Person mismatch was ingested")
	}
}

func TestWSAuthRejected(t *testing.T) {
	s := testServer(t)
	_, device, cred := enrollDevice(t, s, "marco@example.com")
	srv := httptest.NewServer(s.logRequests(s.mux))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// No credential → 401, no upgrade.
	if _, resp, err := dialWS(ctx, srv.URL, ""); err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no-cred dial: err=%v resp=%v, want 401", err, resp)
	}
	// Garbage credential → 401.
	if _, resp, err := dialWS(ctx, srv.URL, "not-a-token"); err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad-cred dial: err=%v resp=%v, want 401", err, resp)
	}
	// Revoked device → 401 (the kill switch), even with a structurally valid credential.
	if err := s.store.RevokeDevice(device, time.Now().UTC()); err != nil {
		t.Fatalf("RevokeDevice: %v", err)
	}
	if _, resp, err := dialWS(ctx, srv.URL, cred); err == nil || resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked dial: err=%v resp=%v, want 401", err, resp)
	}
}
