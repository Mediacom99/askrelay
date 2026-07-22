package envelope

import (
	"crypto/ed25519"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestVerifyHappyPath(t *testing.T) {
	pub, priv := keypair(t)
	e := sampleEnvelope()
	if err := Sign(&e, priv); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if len(e.Sig) != ed25519.SignatureSize {
		t.Fatalf("Sig len = %d, want %d", len(e.Sig), ed25519.SignatureSize)
	}
	if err := Verify(e, pub); err != nil {
		t.Errorf("Verify a freshly signed envelope: %v", err)
	}
}

// TestVerifyTamperMatrix mutates each signed field in turn and requires
// verification to fail — with the specific verdict for version (checked before
// the signature) and ErrBadSignature for everything the signature covers.
func TestVerifyTamperMatrix(t *testing.T) {
	pub, priv := keypair(t)
	cases := []struct {
		name   string
		mutate func(*Envelope)
		want   error
	}{
		{"id", func(e *Envelope) { e.ID = "tampered" }, ErrBadSignature},
		{"thread", func(e *Envelope) { e.Thread = "tampered" }, ErrBadSignature},
		{"from.person", func(e *Envelope) { e.From.Person = "mallory" }, ErrBadSignature},
		{"from.device", func(e *Envelope) { e.From.Device = "SHA256:evil" }, ErrBadSignature},
		{"from.agent", func(e *Envelope) { e.From.Agent = "evil-agent" }, ErrBadSignature},
		{"to", func(e *Envelope) { e.To = "eve" }, ErrBadSignature},
		{"state", func(e *Envelope) { e.State = StateWorking }, ErrBadSignature},
		{"sent_at", func(e *Envelope) { e.SentAt = e.SentAt.Add(time.Second) }, ErrBadSignature},
		{"ai_generated", func(e *Envelope) { e.AIGenerated = !e.AIGenerated }, ErrBadSignature},
		{"body.role", func(e *Envelope) { e.Body.Role = "user" }, ErrBadSignature},
		{"body.part.type", func(e *Envelope) { e.Body.Parts[0].Type = "code" }, ErrBadSignature},
		{"body.part.text", func(e *Envelope) { e.Body.Parts[0].Text = "tampered text" }, ErrBadSignature},
		{"body.add-part", func(e *Envelope) {
			e.Body.Parts = append(e.Body.Parts, Part{Type: "text", Text: "extra"})
		}, ErrBadSignature},
		{"version", func(e *Envelope) { e.V = 2 }, ErrBadVersion},
		{"sig.bitflip", func(e *Envelope) { e.Sig[0] ^= 0x01 }, ErrBadSignature},
		{"sig.truncate", func(e *Envelope) { e.Sig = e.Sig[:len(e.Sig)-1] }, ErrBadSignature},
		{"sig.empty", func(e *Envelope) { e.Sig = nil }, ErrBadSignature},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := sampleEnvelope()
			if err := Sign(&e, priv); err != nil {
				t.Fatalf("Sign: %v", err)
			}
			c.mutate(&e)
			if err := Verify(e, pub); !errors.Is(err, c.want) {
				t.Errorf("Verify after tampering %s = %v, want %v", c.name, err, c.want)
			}
		})
	}
}

func TestVerifyWrongKey(t *testing.T) {
	_, priv := keypair(t)
	e := sampleEnvelope()
	if err := Sign(&e, priv); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	// A different key must not verify.
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(200 - i)
	}
	otherPub := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	if err := Verify(e, otherPub); !errors.Is(err, ErrBadSignature) {
		t.Errorf("Verify with wrong key = %v, want ErrBadSignature", err)
	}
}

func TestVerifyBadPublicKeyLength(t *testing.T) {
	_, priv := keypair(t)
	e := sampleEnvelope()
	if err := Sign(&e, priv); err != nil {
		t.Fatalf("Sign: %v", err)
	}
	// A malformed public key must be a signature verdict, never a panic.
	if err := Verify(e, ed25519.PublicKey{1, 2, 3}); !errors.Is(err, ErrBadSignature) {
		t.Errorf("Verify with short pubkey = %v, want ErrBadSignature", err)
	}
	if err := Verify(e, nil); !errors.Is(err, ErrBadSignature) {
		t.Errorf("Verify with nil pubkey = %v, want ErrBadSignature", err)
	}
}

func TestSignBadPrivateKeyLength(t *testing.T) {
	e := sampleEnvelope()
	if err := Sign(&e, ed25519.PrivateKey{1, 2, 3}); err == nil {
		t.Errorf("Sign with short privkey should error")
	}
	if len(e.Sig) != 0 {
		t.Errorf("failed Sign should not set Sig")
	}
}

func TestSignRejectsBadVersion(t *testing.T) {
	_, priv := keypair(t)
	e := sampleEnvelope()
	e.V = 2
	if err := Sign(&e, priv); !errors.Is(err, ErrBadVersion) {
		t.Errorf("Sign(v=2) = %v, want ErrBadVersion", err)
	}
}

func TestSignSizeCap(t *testing.T) {
	_, priv := keypair(t)

	// Exactly at the cap: signs fine.
	atCap := sampleEnvelope()
	atCap.Body.Parts = []Part{{Type: "text", Text: strings.Repeat("a", MaxBodyBytes)}}
	if err := Sign(&atCap, priv); err != nil {
		t.Errorf("Sign at cap = %v, want nil", err)
	}

	// One byte over: ErrTooLarge.
	over := sampleEnvelope()
	over.Body.Parts = []Part{{Type: "text", Text: strings.Repeat("a", MaxBodyBytes+1)}}
	if err := Sign(&over, priv); !errors.Is(err, ErrTooLarge) {
		t.Errorf("Sign over cap = %v, want ErrTooLarge", err)
	}

	// Cap is the sum across parts, not per part.
	spread := sampleEnvelope()
	spread.Body.Parts = []Part{
		{Type: "text", Text: strings.Repeat("a", MaxBodyBytes/2)},
		{Type: "text", Text: strings.Repeat("b", MaxBodyBytes/2+1)},
	}
	if err := Sign(&spread, priv); !errors.Is(err, ErrTooLarge) {
		t.Errorf("Sign spread over cap = %v, want ErrTooLarge", err)
	}
}

func TestVerifySizeCap(t *testing.T) {
	pub, priv := keypair(t)
	e := sampleEnvelope()
	e.Body.Parts = []Part{{Type: "text", Text: strings.Repeat("a", MaxBodyBytes)}}
	if err := Sign(&e, priv); err != nil {
		t.Fatalf("Sign at cap: %v", err)
	}
	if err := Verify(e, pub); err != nil {
		t.Errorf("Verify at cap = %v, want nil", err)
	}
	// Bloat the body past the cap after signing: caps are enforced before the
	// signature, so this is ErrTooLarge, not ErrBadSignature.
	e.Body.Parts = append(e.Body.Parts, Part{Type: "text", Text: "x"})
	if err := Verify(e, pub); !errors.Is(err, ErrTooLarge) {
		t.Errorf("Verify over cap = %v, want ErrTooLarge", err)
	}
}

// TestPartCountCap exercises the MaxParts boundary (T-16): exactly MaxParts
// parts sign and verify; one more is rejected with ErrTooManyParts in both the
// Sign and the Verify path.
func TestPartCountCap(t *testing.T) {
	pub, priv := keypair(t)
	mk := func(n int) Envelope {
		e := sampleEnvelope()
		parts := make([]Part, n)
		for i := range parts {
			parts[i] = Part{Type: "text", Text: "x"}
		}
		e.Body.Parts = parts
		return e
	}

	// Exactly at the cap: fine.
	at := mk(MaxParts)
	if err := Sign(&at, priv); err != nil {
		t.Errorf("Sign with %d parts = %v, want nil", MaxParts, err)
	}
	if err := Verify(at, pub); err != nil {
		t.Errorf("Verify with %d parts = %v, want nil", MaxParts, err)
	}

	// One over the cap: ErrTooManyParts on Sign.
	over := mk(MaxParts + 1)
	if err := Sign(&over, priv); !errors.Is(err, ErrTooManyParts) {
		t.Errorf("Sign with %d parts = %v, want ErrTooManyParts", MaxParts+1, err)
	}

	// Append a part to the signed at-limit envelope: Verify rejects on the
	// part-count cap (before the signature would fail on the mutated body).
	overV := at
	overV.Body.Parts = append(append([]Part{}, at.Body.Parts...), Part{Type: "text", Text: "x"})
	if err := Verify(overV, pub); !errors.Is(err, ErrTooManyParts) {
		t.Errorf("Verify with %d parts = %v, want ErrTooManyParts", MaxParts+1, err)
	}
}

// TestWireSizeCap exercises the MaxWireBytes boundary (T-16) via metadata bloat
// in a non-body field (so the body-text cap is not what fires): a canonical
// envelope of exactly MaxWireBytes signs and verifies; one byte larger is
// rejected with ErrTooLarge. The padding is ASCII so canonical length grows
// one-for-one and the boundary is exact.
func TestWireSizeCap(t *testing.T) {
	pub, priv := keypair(t)
	mk := func(pad int) Envelope {
		e := sampleEnvelope()
		e.From.Agent = strings.Repeat("a", pad) // metadata bloat, no body-text growth
		e.Body.Parts = []Part{{Type: "text", Text: "x"}}
		return e
	}

	// Measure the canonical size with no padding, then pad to land exactly on
	// MaxWireBytes.
	baseCanon, err := Canonical(mk(0))
	if err != nil {
		t.Fatalf("Canonical(base): %v", err)
	}
	pad := MaxWireBytes - len(baseCanon)
	if pad <= 0 {
		t.Fatalf("baseline canonical already %d bytes; cannot pad up to cap", len(baseCanon))
	}

	at := mk(pad)
	atCanon, err := Canonical(at)
	if err != nil {
		t.Fatalf("Canonical(at): %v", err)
	}
	if len(atCanon) != MaxWireBytes {
		t.Fatalf("padding math off: canonical = %d bytes, want exactly %d", len(atCanon), MaxWireBytes)
	}
	if err := Sign(&at, priv); err != nil {
		t.Errorf("Sign at wire cap (%d bytes) = %v, want nil", MaxWireBytes, err)
	}
	if err := Verify(at, pub); err != nil {
		t.Errorf("Verify at wire cap = %v, want nil", err)
	}

	// One byte over: ErrTooLarge on Sign.
	over := mk(pad + 1)
	if err := Sign(&over, priv); !errors.Is(err, ErrTooLarge) {
		t.Errorf("Sign one byte over wire cap = %v, want ErrTooLarge", err)
	}

	// Verify path: bloat the signed at-limit envelope by one byte of metadata.
	overV := at
	overV.From.Agent = strings.Repeat("a", pad+1)
	if err := Verify(overV, pub); !errors.Is(err, ErrTooLarge) {
		t.Errorf("Verify one byte over wire cap = %v, want ErrTooLarge", err)
	}
}
