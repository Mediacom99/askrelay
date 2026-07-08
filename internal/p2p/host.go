// Package p2p brings up the libp2p networking layer for pchat.
//
// host.go covers libp2p host setup: TCP + QUIC transports, Noise security, NAT
// config (AutoNAT, DCUtR hole punching, circuit relay v2), and connection manager
// watermarks.
//
// TODO(pchat): implement libp2p host setup per pchat-architecture.md §3 (Networking).
// See build order §10 step 3.
package p2p
