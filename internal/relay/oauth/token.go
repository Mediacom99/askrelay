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
// Revocation within a token's lifetime is enforced separately by the bearer
// middleware's live device check, not here.
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

// loadOrCreateKey reads a raw ed25519 private key from path, or generates and
// writes one (0600) if the file does not exist. A wrong-length file is an
// error rather than a silently-truncated key.
func loadOrCreateKey(path string) (ed25519.PrivateKey, error) {
	switch b, err := os.ReadFile(path); {
	case err == nil:
		if len(b) != ed25519.PrivateKeySize {
			return nil, fmt.Errorf("oauth: signing key %q is %d bytes, want a %d-byte ed25519 key", path, len(b), ed25519.PrivateKeySize)
		}
		return ed25519.PrivateKey(b), nil
	case errors.Is(err, os.ErrNotExist):
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("oauth: generate signing key: %w", err)
		}
		if err := os.WriteFile(path, priv, 0o600); err != nil {
			return nil, fmt.Errorf("oauth: write signing key: %w", err)
		}
		return priv, nil
	default:
		return nil, fmt.Errorf("oauth: read signing key: %w", err)
	}
}
