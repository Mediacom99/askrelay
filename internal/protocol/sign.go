// Package protocol — signing helpers.
//
// sign.go covers signing a message envelope with the session private key and
// verifying the signature before a peer renders or relays a message.
//
// TODO(pchat): implement sign/verify helpers per pchat-architecture.md §4 (Signing)
// and §7 (Security & Threat Model). See build order §10 step 2.
package protocol
