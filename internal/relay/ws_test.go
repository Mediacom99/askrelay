package relay

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

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
