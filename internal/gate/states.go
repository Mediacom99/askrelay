package gate

// Direction is which way an approvable flows relative to the person who must
// approve it. It is part of a grant's identity (grants are per-thread ×
// direction) and appears in audit rows, so it is a self-describing string, not
// an opaque integer.
type Direction string

const (
	Inbound  Direction = "inbound"  // a message arriving for the person to act on
	Outbound Direction = "outbound" // a reply their AI drafted, awaiting release
)

// Kind identifies what is being approved. v1 has exactly one kind; the type
// exists so gates, grants, and audit rows never assume message-ness — the hinge
// that lets the same machine gate other actions later (D-19/C4).
type Kind string

const KindMessage Kind = "message"

// Approvable is the unit the gate reasons about: a payload of some kind, flowing
// in a direction, within a thread. The gate never inspects Payload — it only
// decides whether it may proceed — which keeps the machine reusable beyond
// messages. The current lifecycle state is deliberately NOT held here: the gate
// is I/O-free, so the caller passes the authoritative current state in at
// transition time (the store owns that truth, never the sender).
type Approvable struct {
	Kind      Kind
	Direction Direction
	Thread    string // thread id this approvable belongs to
	Payload   any    // opaque to the gate; an envelope.Envelope in v1
}

// DraftState is the lifecycle of an OUTBOUND approvable — a reply awaiting
// release. It is deliberately distinct from a2a.ThreadState: a draft is
// gate-internal bookkeeping, not a state on the A2A task the wire knows about.
// String-typed for the same audit-row reason as Direction.
type DraftState string

const (
	PendingReview DraftState = "pending_review" // drafted, awaiting the human
	Sent          DraftState = "sent"           // released to the recipient
	Discarded     DraftState = "discarded"      // rejected by the human
)
