// Package protocol defines the pchat wire format: the shared CBOR message envelope
// (v, type, sender, ts, sig, payload) and the type-specific payloads (chat, presence,
// typing), plus their encode/decode helpers.
//
// TODO(pchat): implement the Message envelope and CBOR encode/decode per
// pchat-architecture.md §4 (Wire Protocol). See build order §10 step 2.
package protocol
