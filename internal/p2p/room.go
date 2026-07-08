// Package p2p — rooms.
//
// room.go covers topic derivation (topic_id = SHA-256(room_passphrase)) and
// gossipsub join/leave / publish / subscribe for a room.
//
// TODO(pchat): implement topic derivation and gossipsub join/leave per
// pchat-architecture.md §3 (Networking, Rooms). See build order §10 step 4.
package p2p
