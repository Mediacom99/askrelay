package envelope

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// sampleEnvelope returns a deterministic, in-spec envelope for tests. It does
// not use New (which mints random ids and the current time) so canonical output
// is stable across runs.
func sampleEnvelope() Envelope {
	return Envelope{
		V:      Version,
		ID:     "01903b8f-0000-7000-8000-000000000001",
		Thread: "01903b8f-0000-7000-8000-000000000000",
		From: Party{
			Person: "edo",
			Device: "SHA256:abc",
			Agent:  "claude-code",
		},
		To:          "marco",
		State:       StateSubmitted,
		SentAt:      time.Date(2026, 7, 16, 9, 30, 0, 0, time.UTC),
		AIGenerated: true,
		Body: Message{
			Role:  "agent",
			Parts: []Part{{Type: "text", Text: "how does the auth flow work?"}},
		},
	}
}

// keypair returns a fixed Ed25519 keypair derived from a constant seed so tests
// are deterministic.
func keypair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	t.Helper()
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	return priv.Public().(ed25519.PublicKey), priv
}

func TestNewDefaults(t *testing.T) {
	body := Message{Role: "user", Parts: []Part{{Type: "text", Text: "hi"}}}
	before := time.Now().UTC()
	e := New(Party{Person: "edo", Device: "d", Agent: "claude-code"}, "marco", "", StateSubmitted, false, body)
	after := time.Now().UTC()

	if e.V != Version {
		t.Errorf("V = %d, want %d", e.V, Version)
	}
	if e.ID == "" || e.Thread == "" {
		t.Errorf("New left empty ids: id=%q thread=%q", e.ID, e.Thread)
	}
	if e.ID == e.Thread {
		t.Errorf("id and freshly-minted thread should differ: %q", e.ID)
	}
	if e.SentAt.Location() != time.UTC {
		t.Errorf("SentAt not UTC: %v", e.SentAt.Location())
	}
	if e.SentAt.Before(before) || e.SentAt.After(after) {
		t.Errorf("SentAt %v outside [%v,%v]", e.SentAt, before, after)
	}
	if len(e.Sig) != 0 {
		t.Errorf("New should not sign; Sig len = %d", len(e.Sig))
	}
	if e.To != "marco" || e.State != StateSubmitted || e.AIGenerated {
		t.Errorf("New copied fields wrong: %+v", e)
	}
}

func TestNewUUIDv7Sortable(t *testing.T) {
	// UUIDv7 ids are time-ordered; ids minted in sequence should sort ascending.
	body := Message{Role: "user", Parts: []Part{{Type: "text", Text: "x"}}}
	prev := ""
	for range 50 {
		e := New(Party{}, "to", "", StateSubmitted, false, body)
		if prev != "" && e.ID < prev {
			t.Fatalf("UUIDv7 ids not monotonic: %q came after %q", e.ID, prev)
		}
		prev = e.ID
	}
}

func TestNewReuseThread(t *testing.T) {
	body := Message{Role: "user", Parts: []Part{{Type: "text", Text: "reply"}}}
	e := New(Party{}, "marco", "thread-123", StateWorking, true, body)
	if e.Thread != "thread-123" {
		t.Errorf("thread not reused: %q", e.Thread)
	}
}

func TestDecodeRoundTrip(t *testing.T) {
	_, priv := keypair(t)
	e := sampleEnvelope()
	if err := Sign(&e, priv); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	// Canonical forms must match across the encode/decode round trip so a
	// received envelope verifies against what the sender signed.
	cWant, err := Canonical(e)
	if err != nil {
		t.Fatalf("Canonical(e): %v", err)
	}
	cGot, err := Canonical(got)
	if err != nil {
		t.Fatalf("Canonical(got): %v", err)
	}
	if string(cWant) != string(cGot) {
		t.Errorf("canonical form changed across round trip:\n want: %s\n got:  %s", cWant, cGot)
	}
	if string(got.Sig) != string(e.Sig) {
		t.Errorf("signature changed across round trip")
	}
}

func TestDecodeIgnoresUnknownFields(t *testing.T) {
	// §12: unknown fields are ignored for forward compatibility.
	raw := `{"v":1,"id":"a","thread":"b","to":"marco","state":"submitted",` +
		`"sent_at":"2026-07-16T09:30:00Z","ai_generated":false,` +
		`"body":{"role":"user","parts":[{"type":"text","text":"hi"}]},` +
		`"future_field":{"nested":true},"another":42}`
	e, err := Decode([]byte(raw))
	if err != nil {
		t.Fatalf("Decode rejected unknown fields: %v", err)
	}
	if e.To != "marco" || e.Body.Parts[0].Text != "hi" {
		t.Errorf("known fields not decoded: %+v", e)
	}
}

func TestDecodeRejectsUnknownVersion(t *testing.T) {
	raw := `{"v":999,"id":"a","thread":"b","to":"marco","state":"submitted",` +
		`"sent_at":"2026-07-16T09:30:00Z","ai_generated":false,` +
		`"body":{"role":"user","parts":[]}}`
	_, err := Decode([]byte(raw))
	if !errors.Is(err, ErrBadVersion) {
		t.Errorf("Decode(v=999) err = %v, want ErrBadVersion", err)
	}
}

func TestDecodeRejectsTrailingData(t *testing.T) {
	raw := `{"v":1,"id":"a","thread":"b","to":"m","state":"submitted",` +
		`"sent_at":"2026-07-16T09:30:00Z","ai_generated":false,` +
		`"body":{"role":"user","parts":[]}} trailing`
	_, err := Decode([]byte(raw))
	if err == nil {
		t.Errorf("Decode accepted trailing data")
	}
	if errors.Is(err, ErrBadVersion) {
		t.Errorf("wrong error for trailing data: %v", err)
	}
}

func TestDecodeRejectsGarbage(t *testing.T) {
	for _, in := range []string{``, `{`, `not json`, `[1,2,3`, `"unterminated`} {
		if _, err := Decode([]byte(in)); err == nil {
			t.Errorf("Decode(%q) accepted invalid input", in)
		}
	}
}

func TestBodySize(t *testing.T) {
	m := Message{Parts: []Part{
		{Type: "text", Text: "abc"},
		{Type: "text", Text: "de"},
	}}
	if got := bodySize(m); got != 5 {
		t.Errorf("bodySize = %d, want 5", got)
	}
	// Multibyte text is measured in UTF-8 bytes, not runes.
	m2 := Message{Parts: []Part{{Type: "text", Text: "€"}}} // 3 bytes
	if got := bodySize(m2); got != 3 {
		t.Errorf("bodySize(€) = %d, want 3", got)
	}
}

func TestVersionConstant(t *testing.T) {
	if Version != 1 {
		t.Errorf("Version = %d, want 1 (bump is an append-only, documented change)", Version)
	}
	if MaxBodyBytes != 32*1024 {
		t.Errorf("MaxBodyBytes = %d, want 32768", MaxBodyBytes)
	}
}

// TestStateNamesVerbatim guards the A2A-verbatim hyphenated wire spellings.
func TestStateNamesVerbatim(t *testing.T) {
	pairs := map[ThreadState]string{
		StateSubmitted:     "submitted",
		StateWorking:       "working",
		StateInputRequired: "input-required",
		StateCompleted:     "completed",
		StateFailed:        "failed",
		StateCanceled:      "canceled",
		StateRejected:      "rejected",
	}
	for got, want := range pairs {
		if string(got) != want {
			t.Errorf("state constant = %q, want %q", got, want)
		}
	}
	// The state marshals to its verbatim string on the wire.
	e := sampleEnvelope()
	e.State = StateInputRequired
	raw, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"state":"input-required"`) {
		t.Errorf("state not serialized verbatim: %s", raw)
	}
}
