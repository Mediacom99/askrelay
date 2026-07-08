// Package ui is the terminal UI for pchat, built on Bubbletea + Lipgloss.
//
// model.go is the Bubbletea root model: state, Update/View, SIGWINCH resize handling,
// and wiring to the p2p layer (send input → publish; receive → append to scrollback).
//
// TODO(pchat): implement the Bubbletea root model per pchat-architecture.md §5
// (Terminal UI). See build order §10 step 6.
package ui
