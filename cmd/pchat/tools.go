//go:build tools

// This file anchors pchat's core third-party dependencies in go.mod / go.sum before
// the packages that will use them are implemented, so `go mod tidy` keeps them rather
// than pruning them as unused. It is guarded by the `tools` build tag and is therefore
// never compiled into any real build of pchat. Delete it once these libraries are
// imported by real code (see build order in pchat-architecture.md §10).
//
// TODO(pchat): remove once identity/protocol/p2p/ui import these directly.
package main

import (
	_ "github.com/charmbracelet/bubbletea" // §5 Terminal UI
	_ "github.com/charmbracelet/lipgloss"  // §5 Terminal UI styling
	_ "github.com/fxamacker/cbor/v2"       // §4 Wire Protocol (CBOR)
	_ "github.com/libp2p/go-libp2p"        // §3 Networking (libp2p host)
	_ "github.com/libp2p/go-libp2p-pubsub" // §3 Networking (gossipsub)
	// §3 Networking (rendezvous discovery). The canonical libp2p/go-libp2p-rendezvous
	// module is dead upstream (master stripped of code; only 2019 pre-modules code
	// remains), so we use the actively maintained Waku fork, which tracks modern
	// go-libp2p. See CLAUDE.md and pchat-architecture.md §3.
	_ "github.com/waku-org/go-libp2p-rendezvous"
)
