package oauth

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testAud = "https://relay.example.com"

func testIssuer(t *testing.T) *Issuer {
	t.Helper()
	iss, err := NewIssuer(filepath.Join(t.TempDir(), "signing.key"), testAud, time.Hour)
	if err != nil {
		t.Fatalf("NewIssuer: %v", err)
	}
	return iss
}

func TestMintVerifyRoundTrip(t *testing.T) {
	iss := testIssuer(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	tok, err := iss.Mint("person-1", "claude.ai", now)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	person, client, err := iss.Verify(tok, now)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if person != "person-1" || client != "claude.ai" {
		t.Errorf("claims = (%q, %q), want (person-1, claude.ai)", person, client)
	}
}

func TestVerifyRejects(t *testing.T) {
	iss := testIssuer(t)
	now := time.Unix(1_700_000_000, 0).UTC()

	// Token minted by a DIFFERENT signing key.
	other := testIssuer(t)
	otherTok, _ := other.Mint("p", "c", now)
	if _, _, err := iss.Verify(otherTok, now); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("foreign-key token: err = %v, want ErrInvalidToken", err)
	}

	// Wrong audience/issuer (a token whose aud is some other resource).
	wrongAud, err := NewIssuer(filepath.Join(t.TempDir(), "k"), "https://evil.example.com", time.Hour)
	if err != nil {
		t.Fatalf("NewIssuer: %v", err)
	}
	if _, _, err := iss.Verify(mustMint(t, wrongAud, "p", now), now); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("wrong-audience token: err = %v, want ErrInvalidToken", err)
	}

	// Expired: minted 2h ago, verified now.
	expired, _ := iss.Mint("p", "c", now.Add(-2*time.Hour))
	if _, _, err := iss.Verify(expired, now); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expired token: err = %v, want ErrInvalidToken", err)
	}

	// Garbage.
	if _, _, err := iss.Verify("not.a.jwt", now); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("garbage token: err = %v, want ErrInvalidToken", err)
	}
}

// TestVerifyRejectsAlgConfusion forges a token with the alg swapped to HS256
// keyed on the relay's PUBLIC key — the classic JWT confusion attack — and
// confirms WithValidMethods refuses it.
func TestVerifyRejectsAlgConfusion(t *testing.T) {
	iss := testIssuer(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	claims := AccessClaims{RegisteredClaims: jwt.RegisteredClaims{
		Subject: "attacker", Issuer: testAud, Audience: jwt.ClaimStrings{testAud},
		ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
	}}
	// HMAC-sign using the public key bytes as the HMAC secret.
	forged, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(iss.pub))
	if err != nil {
		t.Fatalf("forge: %v", err)
	}
	if _, _, err := iss.Verify(forged, now); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("HS256-confusion forgery accepted: err = %v, want ErrInvalidToken", err)
	}
}

func TestLoadOrCreateKeyPersistsAndValidates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "signing.key")

	k1, err := loadOrCreateKey(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("key file mode = %v, want 0600", info.Mode().Perm())
	}
	// Reloading returns the SAME key (persistence across restart).
	k2, err := loadOrCreateKey(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !k1.Equal(k2) {
		t.Error("reloaded key differs from the generated one")
	}
	// A wrong-length file is refused, not silently accepted.
	bad := filepath.Join(t.TempDir(), "bad.key")
	if err := os.WriteFile(bad, []byte("too-short"), 0o600); err != nil {
		t.Fatalf("write bad: %v", err)
	}
	if _, err := loadOrCreateKey(bad); err == nil {
		t.Error("loadOrCreateKey accepted a wrong-length key file")
	}
}

func mustMint(t *testing.T, iss *Issuer, person string, now time.Time) string {
	t.Helper()
	tok, err := iss.Mint(person, "c", now)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	return tok
}
