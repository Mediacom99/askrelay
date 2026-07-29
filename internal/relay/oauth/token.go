package oauth

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken is returned by Verify for any token that fails signature,
// audience, issuer, or expiry checks. Callers match it with errors.Is; the
// wrapped detail stays server-side (never returned to a remote caller).
var ErrInvalidToken = errors.New("oauth: invalid token")

// Token uses. The use claim makes the two token types non-interchangeable: a
// long-lived device credential must not be replayable as a short-lived access
// bearer token, and vice versa.
const (
	useAccess = "access"
	useDevice = "device"
)

// Issuer mints and verifies the relay's EdDSA-signed access tokens (T-06).
// Tokens are stateless: verification is an offline signature + claim check.
// Audience and issuer are both the relay base URL (RFC 8707 resource binding).
//
// Access tokens are person-scoped and are revoked ONLY by their short (1h)
// lifetime — there is no per-request device check here, and access tokens
// carry no device claim. Immediate revocation applies to device credentials
// (the live store.ActiveDeviceByID check the /ws path performs, WP-08) and to
// new-token issuance (the AS re-checks the device on every mint/refresh,
// WP-06); an already-issued access token stays valid until it expires.
type Issuer struct {
	priv     ed25519.PrivateKey
	pub      ed25519.PublicKey
	audience string
	ttl      time.Duration
}

// AccessClaims is the JWT payload for both token types: the person behind the
// request, the token use (access | device), the device id (on device
// credentials), and the originating client type (which drives the §5.4
// profiles at WP-07).
type AccessClaims struct {
	jwt.RegisteredClaims
	Use        string `json:"use"`
	Device     string `json:"device,omitempty"`
	ClientType string `json:"client_type,omitempty"`
}

// NewIssuer loads the Ed25519 signing key from keyPath, generating and writing
// it (0600) if absent so tokens survive a relay restart. audience is the relay
// base URL; ttl is the access-token lifetime (1h per T-06).
func NewIssuer(keyPath, audience string, ttl time.Duration) (*Issuer, error) {
	priv, err := loadOrCreateKey(keyPath)
	if err != nil {
		return nil, err
	}
	return &Issuer{
		priv:     priv,
		pub:      priv.Public().(ed25519.PublicKey),
		audience: audience,
		ttl:      ttl,
	}, nil
}

// Mint returns a signed access token (use="access") for personID / clientType,
// valid for the issuer's ttl from now. now is injected so expiry is testable.
func (i *Issuer) Mint(personID, clientType string, now time.Time) (string, error) {
	return i.sign(AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   personID,
			Issuer:    i.audience,
			Audience:  jwt.ClaimStrings{i.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(i.ttl)),
		},
		Use:        useAccess,
		ClientType: clientType,
	})
}

// Verify checks an access token — signature, audience, issuer, expiry, and
// use=="access" — returning the person and client type. Entirely offline; now
// is injected (callers pass time.Now()) so expiry is deterministic under test.
// A device credential is refused here (use mismatch).
func (i *Issuer) Verify(token string, now time.Time) (personID, clientType string, err error) {
	claims, err := i.parse(token, now)
	if err != nil {
		return "", "", err
	}
	if claims.Use != useAccess {
		return "", "", fmt.Errorf("%w: not an access token", ErrInvalidToken)
	}
	return claims.Subject, claims.ClientType, nil
}

// sign serializes claims as an EdDSA-signed JWT with the issuer's key.
func (i *Issuer) sign(c AccessClaims) (string, error) {
	tok, err := jwt.NewWithClaims(jwt.SigningMethodEdDSA, c).SignedString(i.priv)
	if err != nil {
		return "", fmt.Errorf("oauth: sign token: %w", err)
	}
	return tok, nil
}

// parse verifies signature (EdDSA only — the allow-list defeats
// algorithm-confusion forgeries), audience, issuer, and expiry as of now, and
// returns the claims. It does NOT check the use claim — each caller enforces
// its own. Any failure collapses to ErrInvalidToken.
func (i *Issuer) parse(token string, now time.Time) (AccessClaims, error) {
	var claims AccessClaims
	_, err := jwt.ParseWithClaims(token, &claims,
		func(*jwt.Token) (any, error) { return i.pub, nil },
		jwt.WithValidMethods([]string{"EdDSA"}),
		jwt.WithAudience(i.audience),
		jwt.WithIssuer(i.audience),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(func() time.Time { return now }),
	)
	if err != nil {
		return AccessClaims{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	return claims, nil
}

// loadOrCreateKey loads the signing key from path, or generates one (0600) on
// first run. Creation is atomic via O_CREATE|O_EXCL: if two processes race a
// first start, exactly one generates the key and the rest adopt it, so they
// can never split-brain onto different keys.
func loadOrCreateKey(path string) (ed25519.PrivateKey, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	switch {
	case err == nil:
		_, priv, gerr := ed25519.GenerateKey(rand.Reader)
		if gerr != nil {
			f.Close()
			_ = os.Remove(path) // don't leave an empty key file behind
			return nil, fmt.Errorf("oauth: generate signing key: %w", gerr)
		}
		if _, werr := f.Write(priv); werr != nil {
			f.Close()
			return nil, fmt.Errorf("oauth: write signing key: %w", werr)
		}
		if cerr := f.Close(); cerr != nil {
			return nil, fmt.Errorf("oauth: write signing key: %w", cerr)
		}
		return priv, nil
	case errors.Is(err, os.ErrExist):
		return readKey(path) // someone else created it (or it predates us)
	default:
		return nil, fmt.Errorf("oauth: create signing key: %w", err)
	}
}

// readKey loads a raw ed25519 private key from path. The public half is
// re-derived from the seed rather than trusted verbatim: a right-length but
// corrupted file otherwise yields a signer whose own tokens never verify (the
// length check alone catches truncation, not in-place corruption).
func readKey(path string) (ed25519.PrivateKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("oauth: read signing key: %w", err)
	}
	if len(b) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("oauth: signing key %q is %d bytes, want a %d-byte ed25519 key", path, len(b), ed25519.PrivateKeySize)
	}
	return ed25519.NewKeyFromSeed(b[:ed25519.SeedSize]), nil
}
