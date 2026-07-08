// Package ui — room view.
//
// room_view.go covers scrollback rendering: the in-memory ring buffer (default cap
// 200), per-line glyph+color prefixes, and the pluggable message renderer.
//
// TODO(pchat): implement scrollback rendering per pchat-architecture.md §5
// (Terminal UI). See build order §10 step 6.
package ui
