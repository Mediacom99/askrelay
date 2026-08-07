package oauth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"

	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

// AS endpoint paths, mounted under the relay base URL. AuthServerMetaPath must
// be at the root well-known location: the go-sdk client discovers AS metadata at
// <base>/.well-known/oauth-authorization-server when the base URL is path-less.
const (
	AuthServerMetaPath = "/.well-known/oauth-authorization-server"
	AuthorizePath      = "/oauth/authorize"
	TokenPath          = "/oauth/token"
	RegisterPath       = "/oauth/register"
	JWKSPath           = "/oauth/jwks"
)

// AuthServerMetadata builds the RFC 8414 authorization-server metadata for this
// relay. Issuer == baseURL == token aud/iss (WP-05). Only S256 PKCE is
// advertised (no "plain": the downgrade is refused, never offered). Clients are
// public (token_endpoint_auth_methods = none); PKCE is the client authentication.
func AuthServerMetadata(baseURL string) *oauthex.AuthServerMeta {
	return &oauthex.AuthServerMeta{
		Issuer:                            baseURL,
		AuthorizationEndpoint:             baseURL + AuthorizePath,
		TokenEndpoint:                     baseURL + TokenPath,
		JWKSURI:                           baseURL + JWKSPath,
		RegistrationEndpoint:              baseURL + RegisterPath,
		ResponseTypesSupported:            []string{"code"},
		GrantTypesSupported:               []string{"authorization_code", "refresh_token"},
		CodeChallengeMethodsSupported:     []string{"S256"},
		TokenEndpointAuthMethodsSupported: []string{"none"},
		ClientIDMetadataDocumentSupported: true,
	}
}

// JWK is a single JSON Web Key (RFC 7517); only the Ed25519 OKP shape is used.
type JWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	Kid string `json:"kid"`
}

// JWKSet is a JWK Set (RFC 7517 §5) — the /oauth/jwks response body.
type JWKSet struct {
	Keys []JWK `json:"keys"`
}

// PublicJWK returns the issuer's Ed25519 verification key as a JWK, with a stable
// RFC 7638 thumbprint as its kid.
func (i *Issuer) PublicJWK() JWK {
	x := base64.RawURLEncoding.EncodeToString(i.pub)
	return JWK{Kty: "OKP", Crv: "Ed25519", X: x, Use: "sig", Alg: "EdDSA", Kid: ed25519Thumbprint(x)}
}

// ed25519Thumbprint is the RFC 7638 JWK thumbprint for an Ed25519 public key
// given its base64url x: SHA-256 over the canonical JSON (members in lexical
// order, no whitespace).
func ed25519Thumbprint(x string) string {
	canon := `{"crv":"Ed25519","kty":"OKP","x":"` + x + `"}`
	sum := sha256.Sum256([]byte(canon))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// VerifyPKCE reports whether codeVerifier matches codeChallenge under S256
// (RFC 7636): BASE64URL(SHA256(verifier)) == challenge. Only S256 is supported;
// "plain" is never accepted (downgrade guard). Constant-time out of habit,
// though the challenge is not itself a secret.
func VerifyPKCE(codeChallenge, codeVerifier string) bool {
	sum := sha256.Sum256([]byte(codeVerifier))
	computed := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(computed), []byte(codeChallenge)) == 1
}
