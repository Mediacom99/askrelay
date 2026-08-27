package daemon

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/Mediacom99/askrelay/internal/a2a"
	"github.com/Mediacom99/askrelay/internal/envelope"
)

func textEnvelope(person, device, text string) envelope.Envelope {
	return envelope.New(
		envelope.Party{Person: person, Device: device}, "p2", "", a2a.StateSubmitted, true,
		envelope.Message{Role: "agent", Parts: []envelope.Part{{Type: "text", Text: text}}},
	)
}

// T-21's core guarantee: the daemon signs ONLY a body this machine forwarded. A
// relay asking for anything else — someone else's text, or a signature under
// another device's name — gets a refusal, because otherwise a compromised relay
// could mint signatures the user never authorised, which is the entire property
// device signatures exist to provide.
func TestSignsOnlyWhatThisMachineForwarded(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	const body = "is staging green?"
	forwarded := textEnvelope("p1", "dev1", body)
	unknown := textEnvelope("p1", "dev1", "text we never sent")
	// Same id as the forwarded draft — so the ONLY reason to refuse is that it
	// names a device that is not ours.
	otherDevice := forwarded
	otherDevice.From.Device = "dev2"

	replies := make(chan frame, 3)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Errorf("accept: %v", err)
			return
		}
		defer func() { _ = c.CloseNow() }()
		for _, e := range []envelope.Envelope{otherDevice, forwarded, unknown} {
			blob, _ := json.Marshal(e)
			req, _ := json.Marshal(frame{Type: "sign_request", ID: e.ID, Envelope: blob})
			if err := c.Write(r.Context(), websocket.MessageText, req); err != nil {
				return
			}
			_, data, err := c.Read(r.Context())
			if err != nil {
				return
			}
			var got frame
			if err := json.Unmarshal(data, &got); err != nil {
				t.Errorf("unmarshal reply: %v", err)
				return
			}
			replies <- got
		}
	}))
	defer srv.Close()

	d := &Daemon{
		cfg:     Config{RelayURL: srv.URL, PersonID: "p1", DeviceID: "dev1", DeviceCredential: "cred"},
		key:     priv,
		log:     slog.New(slog.DiscardHandler),
		seen:    map[string]bool{},
		dir:     t.TempDir(),
		notify:  func() {},
		version: "test",
	}
	d.recordSignable(forwarded.ID, body)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	go func() { _ = d.session(ctx) }()

	next := func(what string) frame {
		t.Helper()
		select {
		case f := <-replies:
			return f
		case <-time.After(5 * time.Second):
			t.Fatalf("no reply to the %s request", what)
			return frame{}
		}
	}

	if f := next("wrong-device"); f.Type != "sign_refused" {
		t.Errorf("wrong device: reply type = %q, want sign_refused", f.Type)
	}

	signedReply := next("forwarded")
	if signedReply.Type != "signature" {
		t.Fatalf("forwarded body: reply type = %q, want signature", signedReply.Type)
	}
	candidate := forwarded
	candidate.Sig = signedReply.Sig
	if err := envelope.Verify(candidate, pub); err != nil {
		t.Errorf("signature does not verify against the device key: %v", err)
	}

	if f := next("never-forwarded"); f.Type != "sign_refused" {
		t.Errorf("unknown body: reply type = %q, want sign_refused", f.Type)
	}

	// Signing consumes the record: the same draft cannot be signed twice.
	if d.forwarded(forwarded.ID, body) {
		t.Error("ledger record survived a successful signature")
	}
}

// The ledger is a cross-process record (T-21), so it must round-trip through
// disk and expire on its own.
func TestLedgerRoundTripAndSweep(t *testing.T) {
	d := &Daemon{log: slog.New(slog.DiscardHandler), dir: t.TempDir()}
	d.recordSignable("draft-1", "hello")

	if !d.forwarded("draft-1", "hello") {
		t.Error("a recorded body was not recognised")
	}
	if d.forwarded("draft-1", "hello!") {
		t.Error("a different body matched the record")
	}
	if d.forwarded("draft-2", "hello") {
		t.Error("an unrecorded draft matched")
	}
	// A draft id must never escape the pending directory.
	d.recordSignable("../../escape", "hello")
	if d.forwarded("../../escape", "hello") {
		t.Error("a path-traversing draft id was accepted")
	}

	d.sweepSignable(time.Now().Add(signableTTL + time.Hour))
	if d.forwarded("draft-1", "hello") {
		t.Error("record survived the sweep past its TTL")
	}
}
