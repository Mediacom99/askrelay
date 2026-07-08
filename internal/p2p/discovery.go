// Package p2p — peer discovery.
//
// discovery.go covers the peer discovery paths: rendezvous (primary), Kademlia DHT
// (resilient fallback), and mDNS (LAN-only), falling through automatically per the
// client discovery order.
//
// TODO(pchat): implement mDNS + DHT + rendezvous discovery per
// pchat-architecture.md §3 (Networking). See build order §10 steps 4–5.
package p2p
