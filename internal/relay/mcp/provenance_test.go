package mcp

import (
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/Mediacom99/askrelay/internal/a2a"
	"github.com/Mediacom99/askrelay/internal/envelope"
	"github.com/Mediacom99/askrelay/internal/relay/store"
)

// The provenance line is the only thing telling a recipient how much a message's
// origin is worth, so it must track reality exactly: verified signature →
// "device verified"; anything else → "relay-attested".
func TestProvenanceOnlyClaimsAVerifiedSignature(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = st.Close() }()

	now := time.Now().UTC()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	token, err := st.CreateInvite("marco@example.com", time.Hour, now)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	person, device, err := st.Enroll(token, pub, "laptop", now)
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}

	h := NewHandler(st, slog.New(slog.NewTextHandler(io.Discard, nil)), "test", nil, nil)
	base := envelope.New(envelope.Party{Person: person.ID, Device: device.ID}, "p2", "", a2a.StateSubmitted, true,
		envelope.Message{Role: "agent", Parts: []envelope.Part{{Type: "text", Text: "hello"}}})

	// Unsigned: the relay is the only attestation there is.
	if got := h.provenance(base, "marco@example.com"); got != "marco@example.com (relay-attested)" {
		t.Errorf("unsigned: provenance = %q", got)
	}

	// Properly signed by the enrolled device.
	signed := base
	if err := envelope.Sign(&signed, priv); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if got := h.provenance(signed, "marco@example.com"); got != "marco@example.com (device verified)" {
		t.Errorf("signed: provenance = %q, want device verified", got)
	}

	// A signature from a key the relay does not hold for that device.
	_, other, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	forged := base
	if err := envelope.Sign(&forged, other); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if got := h.provenance(forged, "marco@example.com"); got != "marco@example.com (relay-attested)" {
		t.Errorf("forged: provenance = %q, want relay-attested", got)
	}

	// Revoking the device stops it vouching for a signature it really did make.
	if err := st.RevokeDevice(device.ID, now); err != nil {
		t.Fatalf("RevokeDevice: %v", err)
	}
	if got := h.provenance(signed, "marco@example.com"); got != "marco@example.com (relay-attested)" {
		t.Errorf("revoked device: provenance = %q, want relay-attested", got)
	}
}
