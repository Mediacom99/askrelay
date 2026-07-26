package store

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "relay.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func newPubkey(t *testing.T) ed25519.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	return pub
}

func TestEnrollHappyPath(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	pub := newPubkey(t)

	token, err := s.CreateInvite("marco@example.com", time.Hour, now)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	p, d, err := s.Enroll(token, pub, "laptop", now)
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}
	if p.Email != "marco@example.com" || p.ID == "" {
		t.Errorf("person = %+v, want non-empty id and matching email", p)
	}
	if d.PersonID != p.ID || d.Label != "laptop" {
		t.Errorf("device = %+v, want PersonID %q and label laptop", d, p.ID)
	}

	got, err := s.ActiveDeviceByPubkey(pub)
	if err != nil {
		t.Fatalf("ActiveDeviceByPubkey: %v", err)
	}
	if got.ID != d.ID || got.PersonID != p.ID {
		t.Errorf("lookup = %+v, want device %q person %q", got, d.ID, p.ID)
	}
}

func TestEnrollInvalidInvites(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()

	if _, _, err := s.Enroll("never-existed", newPubkey(t), "x", now); !errors.Is(err, ErrInviteInvalid) {
		t.Errorf("unknown token: err = %v, want ErrInviteInvalid", err)
	}

	// Second enroll on a consumed token.
	token, err := s.CreateInvite("a@example.com", time.Hour, now)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	if _, _, err := s.Enroll(token, newPubkey(t), "first", now); err != nil {
		t.Fatalf("first Enroll: %v", err)
	}
	if _, _, err := s.Enroll(token, newPubkey(t), "second", now); !errors.Is(err, ErrInviteInvalid) {
		t.Errorf("reused token: err = %v, want ErrInviteInvalid", err)
	}

	// Expired invite.
	expired, err := s.CreateInvite("b@example.com", time.Hour, now)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}
	if _, _, err := s.Enroll(expired, newPubkey(t), "late", now.Add(2*time.Hour)); !errors.Is(err, ErrInviteInvalid) {
		t.Errorf("expired token: err = %v, want ErrInviteInvalid", err)
	}
}

func TestEnrollSingleUseRace(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	token, err := s.CreateInvite("race@example.com", time.Hour, now)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	const n = 64
	var wg sync.WaitGroup
	results := make([]error, n)
	start := make(chan struct{})
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, _, results[i] = s.Enroll(token, newPubkey(t), "dev", now)
		}(i)
	}
	close(start)
	wg.Wait()

	wins := 0
	for _, err := range results {
		switch {
		case err == nil:
			wins++
		case errors.Is(err, ErrInviteInvalid):
		default:
			t.Errorf("unexpected enroll error: %v", err)
		}
	}
	if wins != 1 {
		t.Errorf("winners = %d, want exactly 1 (single-use guard)", wins)
	}
}

func TestRevokeDevice(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	pub := newPubkey(t)
	token, _ := s.CreateInvite("r@example.com", time.Hour, now)
	_, d, err := s.Enroll(token, pub, "laptop", now)
	if err != nil {
		t.Fatalf("Enroll: %v", err)
	}

	if err := s.RevokeDevice(d.ID, now.Add(time.Minute)); err != nil {
		t.Fatalf("RevokeDevice: %v", err)
	}
	if _, err := s.ActiveDeviceByPubkey(pub); !errors.Is(err, ErrNotFound) {
		t.Errorf("post-revoke lookup: err = %v, want ErrNotFound", err)
	}
	// Idempotent: revoking again is a no-op success.
	if err := s.RevokeDevice(d.ID, now.Add(2*time.Minute)); err != nil {
		t.Errorf("second revoke: err = %v, want nil (idempotent)", err)
	}
	// Unknown device id.
	if err := s.RevokeDevice("no-such-device", now); !errors.Is(err, ErrNotFound) {
		t.Errorf("revoke unknown: err = %v, want ErrNotFound", err)
	}
}

func TestEnrollSecondDeviceReusesPerson(t *testing.T) {
	s := newStore(t)
	now := time.Unix(1_700_000_000, 0).UTC()

	t1, _ := s.CreateInvite("same@example.com", time.Hour, now)
	p1, _, err := s.Enroll(t1, newPubkey(t), "phone", now)
	if err != nil {
		t.Fatalf("first Enroll: %v", err)
	}
	t2, _ := s.CreateInvite("same@example.com", time.Hour, now)
	p2, _, err := s.Enroll(t2, newPubkey(t), "laptop", now)
	if err != nil {
		t.Fatalf("second Enroll: %v", err)
	}
	if p1.ID != p2.ID {
		t.Errorf("person ids differ (%q vs %q); same email must reuse the person row", p1.ID, p2.ID)
	}
}
