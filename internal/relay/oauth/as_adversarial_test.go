package oauth

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// ADVERSARIAL (askrelay-test, WP-06 quality pass): the WP-06 security
// obligation "never mint multi-audience access tokens" pinned directly off
// the wire, plus PKCE edge cases (length extremes, unicode/padding, plain
// confusion) beyond as_test.go's RFC 7636 vector.
// ---------------------------------------------------------------------------

// TestMintAlwaysSingleAudience decodes the RAW JWT payload (bypassing
// Verify/parse entirely) and asserts aud has exactly one entry. This is the
// WP-06 obligation from the plan: "Issuer.Verify accepts a token whose aud
// array contains the relay's audience among others (RFC 8707 'one match is
// enough'); keep aud single-valued so a token minted here is not replayable
// at another resource sharing the signing key." Decoding off the wire means
// this still catches a regression even if Verify's own audience check were
// ever loosened.
func TestMintAlwaysSingleAudience(t *testing.T) {
	iss := testIssuer(t)
	tok, err := iss.Mint("person-1", "claude.ai", time.Now().UTC())
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	parts := strings.Split(tok, ".")
	if len(parts) != 3 {
		t.Fatalf("token has %d dot-separated parts, want 3", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var claims struct {
		Aud []string `json:"aud"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}
	if len(claims.Aud) != 1 {
		t.Errorf("aud = %v (%d entries), want exactly 1 -- a multi-audience token minted here would be replayable at another resource sharing this signing key", claims.Aud, len(claims.Aud))
	}
	if len(claims.Aud) > 0 && claims.Aud[0] != testAud {
		t.Errorf("aud[0] = %q, want %q", claims.Aud[0], testAud)
	}
}

// TestMintDeviceCredentialAlsoSingleAudience: the device credential (also
// minted by this issuer, used at /ws) must hold the same invariant.
func TestMintDeviceCredentialAlsoSingleAudience(t *testing.T) {
	iss := testIssuer(t)
	tok, err := iss.MintDeviceCredential("person-1", "device-1", time.Now().UTC())
	if err != nil {
		t.Fatalf("MintDeviceCredential: %v", err)
	}
	parts := strings.Split(tok, ".")
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	var claims struct {
		Aud []string `json:"aud"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		t.Fatalf("unmarshal claims: %v", err)
	}
	if len(claims.Aud) != 1 {
		t.Errorf("aud = %v (%d entries), want exactly 1", claims.Aud, len(claims.Aud))
	}
}

// TestVerifyPKCEEdgeCases extends the RFC 7636 vector in as_test.go with
// length extremes, non-ASCII/binary bytes, and base64url padding tricks. None
// of these should ever match -- the point is confirming VerifyPKCE degrades
// safely (no panic, no accidental match) rather than the exact false result.
func TestVerifyPKCEEdgeCases(t *testing.T) {
	cases := []struct {
		name                string
		challenge, verifier string
	}{
		{"both empty", "", ""},
		{"empty verifier against a real-looking challenge", "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", ""},
		{"empty challenge against a real verifier", "", "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"},
		{"1-char verifier", "x", "a"},
		{"padded base64 challenge (RFC 4648 padding, which S256 challenges must not carry)", "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM=", "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"},
		{"very long matching-length strings", strings.Repeat("z", 100_000), strings.Repeat("z", 100_000)},
		{"non-ASCII / binary verifier", "ch", "日本語🔥\x00\xff"},
		{"verifier below RFC 7636's 43-char minimum", "ch", "short"},
		{"verifier above RFC 7636's 128-char maximum", "ch", strings.Repeat("v", 200)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if VerifyPKCE(c.challenge, c.verifier) {
				t.Errorf("VerifyPKCE(%q, %q) = true, want false (no accidental match)", c.challenge, c.verifier)
			}
		})
	}
}

// FuzzVerifyPKCENoPanic feeds arbitrary byte pairs at VerifyPKCE. It handles
// fully untrusted, attacker-controlled bytes (a code_verifier from a /token
// POST body) so a panic here would be a crafted-request DoS.
func FuzzVerifyPKCENoPanic(f *testing.F) {
	f.Add("E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk")
	f.Add("", "")
	f.Add("ch", "ch")
	f.Add(strings.Repeat("a", 5000), strings.Repeat("b", 5000))
	f.Fuzz(func(t *testing.T, challenge, verifier string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("panic on VerifyPKCE(%q, %q): %v", challenge, verifier, r)
			}
		}()
		_ = VerifyPKCE(challenge, verifier)
	})
}
