package envelope

import (
	"crypto/ed25519"
	"fmt"
)

// Sign computes the Ed25519 signature over the canonical form of e (§3) and
// stores it in e.Sig. It first enforces the version, part-count, and size caps
// (T-16) so a valid signature can never be produced over an out-of-spec
// envelope: an unsupported version returns ErrBadVersion, too many parts
// ErrTooManyParts, and a body-text or whole-wire overrun ErrTooLarge.
func Sign(e *Envelope, priv ed25519.PrivateKey) error {
	if len(priv) != ed25519.PrivateKeySize {
		return fmt.Errorf("envelope: invalid private key length %d", len(priv))
	}
	canon, err := check(*e)
	if err != nil {
		return err
	}
	e.Sig = ed25519.Sign(priv, canon)
	return nil
}

// Verify checks e against pub. It validates, in order, the version
// (ErrBadVersion), the part-count cap (ErrTooManyParts), and the byte caps
// (ErrTooLarge) — all before any signature work — and finally the Ed25519
// signature over the canonical form (ErrBadSignature for a missing, malformed,
// or non-verifying signature, or a wrong-sized public key). A successful return
// proves only that the signing device produced these exact bytes — never that
// the content is safe or was intended for any action (D-10).
func Verify(e Envelope, pub ed25519.PublicKey) error {
	canon, err := check(e)
	if err != nil {
		// Version / cap verdicts (and any canonicalization fault) take
		// precedence over the signature check.
		return err
	}
	if len(pub) != ed25519.PublicKeySize || len(e.Sig) != ed25519.SignatureSize {
		return ErrBadSignature
	}
	if !ed25519.Verify(pub, canon, e.Sig) {
		return ErrBadSignature
	}
	return nil
}
