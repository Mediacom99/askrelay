package store

import (
	"errors"
	"testing"
	"time"
)

// enrollDevice creates a person+device via the invite→enroll path so oauth_codes
// FKs (person_id, device_id) are satisfiable in these tests.
func enrollDevice(t *testing.T, s *Store, email string, now time.Time) (personID, deviceID string) {
	t.Helper()
	token, err := s.CreateInvite(email, time.Hour, now)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	p, d, err := s.Enroll(token, newPubkey(t), "dev", now)
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	return p.ID, d.ID
}

func TestRegisterAndLookupClient(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()

	uris := []string{"https://claude.ai/api/mcp/auth_callback"}
	id, err := s.RegisterClient(uris, "Claude", now)
	if err != nil {
		t.Fatalf("RegisterClient: %v", err)
	}
	if id == "" {
		t.Fatal("RegisterClient returned empty client_id")
	}

	c, err := s.ClientByID(id)
	if err != nil {
		t.Fatalf("ClientByID: %v", err)
	}
	if c.Name != "Claude" || len(c.RedirectURIs) != 1 || c.RedirectURIs[0] != uris[0] {
		t.Errorf("client = %+v, want name Claude and one matching redirect uri", c)
	}

	if _, err := s.ClientByID("no-such-client"); !errors.Is(err, ErrClientUnknown) {
		t.Errorf("unknown client: err = %v, want ErrClientUnknown", err)
	}
}

func TestAuthCodeSingleUse(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	pid, did := enrollDevice(t, s, "marco@example.com", now)

	want := AuthCode{
		ClientID:      "client-x",
		RedirectURI:   "https://claude.ai/cb",
		CodeChallenge: "abc123",
		Resource:      "https://relay.example.com",
		PersonID:      pid,
		DeviceID:      did,
		ClientType:    "claude",
	}
	if err := s.CreateAuthCode("the-code", want, now.Add(time.Minute), now); err != nil {
		t.Fatalf("CreateAuthCode: %v", err)
	}

	// Reading the binding does not consume: it can be read again.
	got, err := s.AuthCodeByCode("the-code", now)
	if err != nil {
		t.Fatalf("AuthCodeByCode: %v", err)
	}
	if got != want {
		t.Errorf("binding = %+v, want %+v", got, want)
	}
	if _, err := s.AuthCodeByCode("the-code", now); err != nil {
		t.Errorf("read must not consume: second read err = %v", err)
	}

	// Claiming it once succeeds; a second claim is refused (single-use guard),
	// and after the claim the binding read is refused too (no oracle).
	if err := s.ConsumeAuthCode("the-code", now); err != nil {
		t.Fatalf("ConsumeAuthCode: %v", err)
	}
	if err := s.ConsumeAuthCode("the-code", now); !errors.Is(err, ErrCodeInvalid) {
		t.Errorf("double claim: err = %v, want ErrCodeInvalid", err)
	}
	if _, err := s.AuthCodeByCode("the-code", now); !errors.Is(err, ErrCodeInvalid) {
		t.Errorf("read after claim: err = %v, want ErrCodeInvalid", err)
	}
}

func TestAuthCodeUnknownAndExpired(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	pid, did := enrollDevice(t, s, "marco@example.com", now)

	if _, err := s.AuthCodeByCode("never-existed", now); !errors.Is(err, ErrCodeInvalid) {
		t.Errorf("unknown code: err = %v, want ErrCodeInvalid", err)
	}

	b := AuthCode{ClientID: "c", RedirectURI: "u", CodeChallenge: "ch", PersonID: pid, DeviceID: did}
	if err := s.CreateAuthCode("stale", b, now.Add(time.Minute), now); err != nil {
		t.Fatalf("CreateAuthCode: %v", err)
	}
	// After expiry → refused, no oracle distinguishing it from unknown, on both
	// the read and the claim.
	if _, err := s.AuthCodeByCode("stale", now.Add(2*time.Minute)); !errors.Is(err, ErrCodeInvalid) {
		t.Errorf("expired read: err = %v, want ErrCodeInvalid", err)
	}
	if err := s.ConsumeAuthCode("stale", now.Add(2*time.Minute)); !errors.Is(err, ErrCodeInvalid) {
		t.Errorf("expired claim: err = %v, want ErrCodeInvalid", err)
	}
}

func TestRefreshRotateAndReuseRevokesFamily(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	pid, did := enrollDevice(t, s, "marco@example.com", now)

	g := RefreshGrant{ClientID: "client-x", PersonID: pid, DeviceID: did, ClientType: "claude", Resource: "https://relay.example.com"}
	// A second live token in the SAME family (person × client) — the successor a
	// legitimate rotation would have minted.
	if err := s.CreateRefreshToken("rt1", g, now.Add(time.Hour), now); err != nil {
		t.Fatalf("CreateRefreshToken rt1: %v", err)
	}
	if err := s.CreateRefreshToken("rt-sibling", g, now.Add(time.Hour), now); err != nil {
		t.Fatalf("CreateRefreshToken rt-sibling: %v", err)
	}

	got, err := s.ConsumeRefreshToken("rt1", now)
	if err != nil {
		t.Fatalf("ConsumeRefreshToken: %v", err)
	}
	if got != g {
		t.Errorf("grant = %+v, want %+v", got, g)
	}

	// Replay of the consumed rt1 → reuse signal AND the family is revoked.
	if _, err := s.ConsumeRefreshToken("rt1", now); !errors.Is(err, ErrRefreshReused) {
		t.Errorf("replay: err = %v, want ErrRefreshReused", err)
	}
	// The still-live sibling is now dead too (family revoke). Presenting it reads
	// as a consumed row → replay, indistinguishable from a real reuse.
	if _, err := s.ConsumeRefreshToken("rt-sibling", now); !errors.Is(err, ErrRefreshReused) {
		t.Errorf("sibling after family revoke: err = %v, want ErrRefreshReused", err)
	}
}

func TestRefreshUnknownAndExpired(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	pid, did := enrollDevice(t, s, "marco@example.com", now)

	if _, err := s.ConsumeRefreshToken("never-existed", now); !errors.Is(err, ErrRefreshInvalid) {
		t.Errorf("unknown refresh: err = %v, want ErrRefreshInvalid", err)
	}

	g := RefreshGrant{ClientID: "c", PersonID: pid, DeviceID: did}
	if err := s.CreateRefreshToken("stale-rt", g, now.Add(time.Hour), now); err != nil {
		t.Fatalf("CreateRefreshToken: %v", err)
	}
	if _, err := s.ConsumeRefreshToken("stale-rt", now.Add(2*time.Hour)); !errors.Is(err, ErrRefreshInvalid) {
		t.Errorf("expired refresh: err = %v, want ErrRefreshInvalid", err)
	}
}

func TestSweepOAuthRemovesExpiredKeepsLive(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	pid, did := enrollDevice(t, s, "marco@example.com", now)

	code := AuthCode{ClientID: "c", RedirectURI: "u", CodeChallenge: "ch", PersonID: pid, DeviceID: did}
	if err := s.CreateAuthCode("expired-code", code, now.Add(time.Minute), now); err != nil {
		t.Fatalf("CreateAuthCode expired: %v", err)
	}
	if err := s.CreateAuthCode("live-code", code, now.Add(time.Hour), now); err != nil {
		t.Fatalf("CreateAuthCode live: %v", err)
	}
	g := RefreshGrant{ClientID: "c", PersonID: pid, DeviceID: did}
	if err := s.CreateRefreshToken("expired-rt", g, now.Add(time.Minute), now); err != nil {
		t.Fatalf("CreateRefreshToken expired: %v", err)
	}
	if err := s.CreateRefreshToken("live-rt", g, now.Add(time.Hour), now); err != nil {
		t.Fatalf("CreateRefreshToken live: %v", err)
	}

	// Sweep at a time past the short-lived pair's expiry but before the live pair.
	n, err := s.SweepOAuth(now.Add(30 * time.Minute))
	if err != nil {
		t.Fatalf("SweepOAuth: %v", err)
	}
	if n != 2 {
		t.Errorf("swept = %d, want 2 (one code + one refresh)", n)
	}
	// The live pair survives.
	if _, err := s.AuthCodeByCode("live-code", now.Add(30*time.Minute)); err != nil {
		t.Errorf("live code swept away: %v", err)
	}
	if _, err := s.ConsumeRefreshToken("live-rt", now.Add(30*time.Minute)); err != nil {
		t.Errorf("live refresh swept away: %v", err)
	}
}
