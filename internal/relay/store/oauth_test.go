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

	got, err := s.ConsumeAuthCode("the-code", now)
	if err != nil {
		t.Fatalf("ConsumeAuthCode: %v", err)
	}
	if got != want {
		t.Errorf("binding = %+v, want %+v", got, want)
	}

	// Second consume of the same code is refused (single-use guard).
	if _, err := s.ConsumeAuthCode("the-code", now); !errors.Is(err, ErrCodeInvalid) {
		t.Errorf("double consume: err = %v, want ErrCodeInvalid", err)
	}
}

func TestAuthCodeUnknownAndExpired(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	pid, did := enrollDevice(t, s, "marco@example.com", now)

	if _, err := s.ConsumeAuthCode("never-existed", now); !errors.Is(err, ErrCodeInvalid) {
		t.Errorf("unknown code: err = %v, want ErrCodeInvalid", err)
	}

	b := AuthCode{ClientID: "c", RedirectURI: "u", CodeChallenge: "ch", PersonID: pid, DeviceID: did}
	if err := s.CreateAuthCode("stale", b, now.Add(time.Minute), now); err != nil {
		t.Fatalf("CreateAuthCode: %v", err)
	}
	// Consume after expiry → refused, no oracle distinguishing it from unknown.
	if _, err := s.ConsumeAuthCode("stale", now.Add(2*time.Minute)); !errors.Is(err, ErrCodeInvalid) {
		t.Errorf("expired code: err = %v, want ErrCodeInvalid", err)
	}
}
