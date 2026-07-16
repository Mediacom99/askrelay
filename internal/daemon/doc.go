// Package daemon implements the optional local component: WebSocket client,
// offline outbound queue, stdio MCP server, Claude Code push paths (channels
// bridge + hooks) (docs/askrelay-architecture.md §6). Implemented by WP-09
// and WP-10. Hard rule: nothing core may require the daemon (D-06).
package daemon
