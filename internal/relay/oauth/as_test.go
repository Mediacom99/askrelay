package oauth

import (
	"crypto/ed25519"
	"encoding/base64"
	"slices"
	"testing"
)

func TestVerifyPKCE(t *testing.T) {
	// RFC 7636 Appendix B test vector.
	const (
		verifier  = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
		challenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	)
	if !VerifyPKCE(challenge, verifier) {
		t.Error("valid S256 verifier rejected")
	}
	if VerifyPKCE(challenge, "wrong-verifier") {
		t.Error("wrong verifier accepted")
	}
	// A "plain" downgrade attempt (verifier presented as the challenge) must fail:
	// only S256 is honored.
	if VerifyPKCE(verifier, verifier) {
		t.Error("plain-method challenge==verifier accepted; only S256 allowed")
	}
}

func TestEd25519Thumbprint(t *testing.T) {
	// RFC 8037 Appendix A.3 test vector.
	const (
		x    = "11qYAYKxCrfVS_7TyWQHOg7hcvPapiMlrwIaaPcHURo"
		want = "kPrK_qmxVWaYVA9wwBF6Iuo3vVzz7TxHCTwXBygrS4k"
	)
	if got := ed25519Thumbprint(x); got != want {
		t.Errorf("thumbprint = %q, want %q", got, want)
	}
}

func TestAuthServerMetadata(t *testing.T) {
	m := AuthServerMetadata(testAud)
	if m.Issuer != testAud {
		t.Errorf("issuer = %q, want %q", m.Issuer, testAud)
	}
	if m.AuthorizationEndpoint != testAud+AuthorizePath ||
		m.TokenEndpoint != testAud+TokenPath ||
		m.JWKSURI != testAud+JWKSPath ||
		m.RegistrationEndpoint != testAud+RegisterPath {
		t.Errorf("endpoints not derived from base URL: %+v", m)
	}
	if !slices.Equal(m.CodeChallengeMethodsSupported, []string{"S256"}) {
		t.Errorf("code_challenge_methods = %v, want [S256] only (no plain)", m.CodeChallengeMethodsSupported)
	}
	if !m.ClientIDMetadataDocumentSupported {
		t.Error("CIMD support must be advertised (ChatGPT path)")
	}
}

func TestPublicJWK(t *testing.T) {
	iss := testIssuer(t)
	jwk := iss.PublicJWK()
	if jwk.Kty != "OKP" || jwk.Crv != "Ed25519" || jwk.Alg != "EdDSA" || jwk.Use != "sig" {
		t.Errorf("jwk shape = %+v, want OKP/Ed25519/EdDSA/sig", jwk)
	}
	raw, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		t.Fatalf("x is not base64url: %v", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		t.Errorf("x decodes to %d bytes, want %d", len(raw), ed25519.PublicKeySize)
	}
	if jwk.Kid == "" {
		t.Error("kid (thumbprint) is empty")
	}
}
