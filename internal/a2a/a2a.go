// Package a2a holds the vocabulary askrelay borrows verbatim from the A2A
// (Agent2Agent) protocol — today, the Task lifecycle states. It is a tiny,
// dependency-free leaf package so every layer that reasons about states (the
// envelope on the wire, the gate's transitions, the store's audit rows) shares
// one source of truth without depending on any of the others.
package a2a

// ThreadState is an A2A Task state, carried verbatim on the wire and in audit
// rows (docs/askrelay-architecture.md §3). The envelope records the state a
// message effects; the gate (WP-02) owns transitions between states.
type ThreadState string

// The seven A2A-verbatim thread states (hyphenated exactly as on the wire).
const (
	StateSubmitted     ThreadState = "submitted"
	StateWorking       ThreadState = "working"
	StateInputRequired ThreadState = "input-required"
	StateCompleted     ThreadState = "completed"
	StateFailed        ThreadState = "failed"
	StateCanceled      ThreadState = "canceled"
	StateRejected      ThreadState = "rejected"
)
