package relay

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/Mediacom99/askrelay/internal/a2a"
	"github.com/Mediacom99/askrelay/internal/envelope"
)

// enrollSigningDevice is enrollDevice, but keeps the private key so the test can
// act as that device's daemon.
func enrollSigningDevice(t *testing.T, s *Server, email string) (personID, deviceID, cred string, priv ed25519.PrivateKey) {
	t.Helper()
	now := time.Now().UTC()
	tok, err := s.store.CreateInvite(email, time.Hour, now)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	person, device, err := s.store.Enroll(tok, pub, "laptop", now)
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	cred, err = s.issuer.MintDeviceCredential(person.ID, device.ID, now)
	if err != nil {
		t.Fatalf("MintDeviceCredential: %v", err)
	}
	return person.ID, device.ID, cred, priv
}

// A connected daemon signs the release, and the relay accepts the signature only
// after verifying it itself (T-21) — it never takes the daemon's word.
func TestSignReleaseWithConnectedDaemon(t *testing.T) {
	s := testServer(t)
	person, deviceID, cred, priv := enrollSigningDevice(t, s, "marco@example.com")
	srv := httptest.NewServer(s.logRequests(s.mux))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _, err := dialWS(ctx, srv.URL, cred)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = c.CloseNow() }()
	if !waitFor(func() bool { return s.hub.count(person) == 1 }) {
		t.Fatal("daemon never registered in the hub")
	}

	// Stand in for the daemon: sign whatever the relay asks, as this device.
	go func() {
		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		var f wsFrame
		if err := json.Unmarshal(data, &f); err != nil || f.Type != "sign_request" {
			return
		}
		var e envelope.Envelope
		if err := json.Unmarshal(f.Envelope, &e); err != nil {
			return
		}
		if err := envelope.Sign(&e, priv); err != nil {
			return
		}
		reply, _ := json.Marshal(wsFrame{Type: "signature", ID: f.ID, Sig: e.Sig})
		_ = c.Write(ctx, websocket.MessageText, reply)
	}()

	e := envelope.New(envelope.Party{Person: person}, "p2", "", a2a.StateSubmitted, true,
		envelope.Message{Role: "agent", Parts: []envelope.Part{{Type: "text", Text: "is staging green?"}}})

	gotDevice, sig, ok := s.SignRelease(ctx, person, e)
	if !ok {
		t.Fatal("SignRelease reported no signature from a connected, cooperating daemon")
	}
	if gotDevice != deviceID {
		t.Errorf("signing device = %q, want %q", gotDevice, deviceID)
	}
	// The envelope the relay verified names the signing device (T-21).
	candidate := e
	candidate.From.Device = gotDevice
	candidate.Sig = sig
	dev, err := s.store.ActiveDeviceByID(deviceID)
	if err != nil {
		t.Fatalf("ActiveDeviceByID: %v", err)
	}
	if err := envelope.Verify(candidate, dev.PubKey); err != nil {
		t.Errorf("returned signature does not verify: %v", err)
	}
}

// No daemon, a refusal, or a bogus signature must all degrade to relay-attested
// rather than block the release (T-21: no liveness coupling).
func TestSignReleaseDegradesGracefully(t *testing.T) {
	s := testServer(t)
	person, _, cred, _ := enrollSigningDevice(t, s, "marco@example.com")
	srv := httptest.NewServer(s.logRequests(s.mux))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	e := envelope.New(envelope.Party{Person: person}, "p2", "", a2a.StateSubmitted, true,
		envelope.Message{Role: "agent", Parts: []envelope.Part{{Type: "text", Text: "hello"}}})

	// 1. Nobody connected at all.
	if _, _, ok := s.SignRelease(ctx, person, e); ok {
		t.Error("signature reported with no daemon connected")
	}

	c, _, err := dialWS(ctx, srv.URL, cred)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = c.CloseNow() }()
	if !waitFor(func() bool { return s.hub.count(person) == 1 }) {
		t.Fatal("daemon never registered in the hub")
	}

	// 2. A daemon that refuses (it never forwarded that body).
	go func() {
		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		var f wsFrame
		if err := json.Unmarshal(data, &f); err != nil {
			return
		}
		reply, _ := json.Marshal(wsFrame{Type: "sign_refused", ID: f.ID})
		_ = c.Write(ctx, websocket.MessageText, reply)
	}()
	if _, _, ok := s.SignRelease(ctx, person, e); ok {
		t.Error("a refusal was treated as a signature")
	}

	// 3. A daemon offering garbage: the relay verifies, so it must not be taken.
	go func() {
		_, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		var f wsFrame
		if err := json.Unmarshal(data, &f); err != nil {
			return
		}
		reply, _ := json.Marshal(wsFrame{Type: "signature", ID: f.ID, Sig: make([]byte, ed25519.SignatureSize)})
		_ = c.Write(ctx, websocket.MessageText, reply)
	}()
	if _, _, ok := s.SignRelease(ctx, person, e); ok {
		t.Error("an invalid signature was accepted — the relay must verify, not trust")
	}
}
