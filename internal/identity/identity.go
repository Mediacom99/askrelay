// Package identity handles the per-session cryptographic identity: fresh Ed25519
// keypair generation (never written to disk), libp2p peer ID derivation, and the
// deterministic glyph+color signature derived from the public key bytes.
//
// TODO(pchat): implement keypair gen and glyph+color derivation per
// pchat-architecture.md §2 (Identity). See build order §10 step 1.
package identity
