package oauth

import (
	"crypto/ed25519"
	"crypto/rand"
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
// TestVerifyRejectsNotYetValid pins that the verifier honors nbf (the WP-05
// "not-yet-valid" test-plan item). Mint never sets NotBefore, so the token is
// crafted directly via the issuer's own signer.
func TestVerifyRejectsNotYetValid(t *testing.T) {
	iss := testIssuer(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	tok, err := iss.sign(AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "p",
			Issuer:    testAud,
			Audience:  jwt.ClaimStrings{testAud},
			NotBefore: jwt.NewNumericDate(now.Add(time.Hour)), // valid only later
			ExpiresAt: jwt.NewNumericDate(now.Add(2 * time.Hour)),
		},
		Use: useAccess,
	})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, _, err := iss.Verify(tok, now); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("not-yet-valid token: err = %v, want ErrInvalidToken", err)
	}
}

// TestVerifyRejectsAudienceMismatchAlone isolates the audience check from the
// signature check: same key and issuer, only the claimed audience differs.
func TestVerifyRejectsAudienceMismatchAlone(t *testing.T) {
	iss := testIssuer(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	tok, err := iss.sign(AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "p",
			Issuer:    testAud,
			Audience:  jwt.ClaimStrings{"https://other.example.com"}, // only this differs
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
		Use: useAccess,
	})
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, _, err := iss.Verify(tok, now); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("wrong-audience token (same key): err = %v, want ErrInvalidToken", err)
	}
}

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

// TestReadKeyReDerivesPublicHalf: a key file with a valid seed but a corrupted
// (zeroed) public half must still produce a self-consistent signer — the pub
// is re-derived from the seed, not trusted verbatim (Finding D).
func TestReadKeyReDerivesPublicHalf(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("keygen: %v", err)
	}
	corrupt := make([]byte, ed25519.PrivateKeySize)
	copy(corrupt, priv[:ed25519.SeedSize]) // keep the seed, leave the pub half zero
	path := filepath.Join(t.TempDir(), "k")
	if err := os.WriteFile(path, corrupt, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	key, err := readKey(path)
	if err != nil {
		t.Fatalf("readKey: %v", err)
	}
	iss := &Issuer{priv: key, pub: key.Public().(ed25519.PublicKey), audience: testAud, ttl: time.Hour}
	now := time.Now().UTC()
	tok, err := iss.Mint("p", "c", now)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if _, _, err := iss.Verify(tok, now); err != nil {
		t.Errorf("signer from re-derived key cannot verify its own token: %v", err)
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
