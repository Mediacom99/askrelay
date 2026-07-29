package oauth

import (
	"errors"
	"testing"
	"time"
)

func TestDeviceCredentialRoundTrip(t *testing.T) {
	iss := testIssuer(t)
	now := time.Unix(1_700_000_000, 0).UTC()
	cred, err := iss.MintDeviceCredential("person-1", "device-9", now)
	if err != nil {
		t.Fatalf("MintDeviceCredential: %v", err)
	}
	person, device, err := iss.VerifyDeviceCredential(cred, now)
	if err != nil {
		t.Fatalf("VerifyDeviceCredential: %v", err)
	}
	if person != "person-1" || device != "device-9" {
		t.Errorf("claims = (%q, %q), want (person-1, device-9)", person, device)
	}
}

// TestTokenUseSeparation is the anti-confusion property: an access token is not
// a device credential and vice versa, even though both are validly signed by
// the same key.
func TestTokenUseSeparation(t *testing.T) {
	iss := testIssuer(t)
	now := time.Unix(1_700_000_000, 0).UTC()

	access, _ := iss.Mint("p", "claude.ai", now)
	device, _ := iss.MintDeviceCredential("p", "d", now)

	if _, _, err := iss.VerifyDeviceCredential(access, now); !errors.Is(err, ErrInvalidToken) {
		t.Error("an access token was accepted as a device credential")
	}
	if _, _, err := iss.Verify(device, now); !errors.Is(err, ErrInvalidToken) {
		t.Error("a device credential was accepted as an access token")
	}
}
