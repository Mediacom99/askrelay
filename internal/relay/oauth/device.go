package oauth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// deviceCredTTL is long: the credential lives with the device, and the real
// kill switch is the live ActiveDeviceByID revocation check at connect time
// (WP-08), not expiry. The long TTL is only a backstop for a lost credential.
const deviceCredTTL = 365 * 24 * time.Hour

// MintDeviceCredential issues the long-lived WS credential (use="device")
// binding personID to deviceID. It is presented on /ws; the caller MUST still
// confirm the device is active via store.ActiveDeviceByID — the token proves
// issuance, the store proves it has not been revoked since.
func (i *Issuer) MintDeviceCredential(personID, deviceID string, now time.Time) (string, error) {
	return i.sign(AccessClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   personID,
			Issuer:    i.audience,
			Audience:  jwt.ClaimStrings{i.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(deviceCredTTL)),
		},
		Use:    useDevice,
		Device: deviceID,
	})
}

// VerifyDeviceCredential checks signature/audience/issuer/expiry and that
// use=="device", returning the person and device ids. Offline; the live
// revocation check is the caller's (WP-08 /ws). An access token is refused
// here (use mismatch).
func (i *Issuer) VerifyDeviceCredential(token string, now time.Time) (personID, deviceID string, err error) {
	claims, err := i.parse(token, now)
	if err != nil {
		return "", "", err
	}
	if claims.Use != useDevice {
		return "", "", fmt.Errorf("%w: not a device credential", ErrInvalidToken)
	}
	return claims.Subject, claims.Device, nil
}
