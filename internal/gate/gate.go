package gate

import (
	"errors"

	"github.com/Mediacom99/askrelay/internal/a2a"
)

// Sentinel errors from the gate's transition functions. Callers branch on these
// with errors.Is; they are the gate's whole vocabulary of refusal.
var (
	// ErrUnknownState means the supplied current state is not one the gate
	// recognizes — an A2A thread state (ApplyInbound) or a draft state
	// (ApplyOutbound). The gate refuses rather than guesses — the F3 guard: a
	// state it does not recognize is never a basis for a transition.
	ErrUnknownState = errors.New("gate: unknown state")
	// ErrIllegalTransition means the verdict is not a legal move from the
	// current state.
	ErrIllegalTransition = errors.New("gate: illegal transition")
	// ErrNoGrant means no active grant covers the approvable, so a human
	// verdict is required — returned by the grant-driven paths when they
	// cannot auto-approve.
	ErrNoGrant = errors.New("gate: no active grant")
	// ErrWrongDirection means the approvable's Direction does not match the
	// path it was routed to — a caller bug the gate refuses rather than
	// trusts: an inbound message must never mint a Release, and an outbound
	// draft must never flip a thread state.
	ErrWrongDirection = errors.New("gate: wrong direction")
)

// Verdict is a person's decision on an approvable awaiting them. The same two
// verdicts apply in both directions; the resulting state differs by direction
// (see ApplyInbound and, later, ApplyOutbound).
type Verdict string

const (
	Approve Verdict = "approve" // let the AI act on it / release the reply
	Reject  Verdict = "reject"  // refuse it
)

// ApplyInbound applies a person's verdict to an inbound message and returns the
// thread state it moves to. It is legal only from input-required — the state a
// delivered-but-unapproved message waits in — so Approve moves the thread to
// working (the recipient's AI may now act) and Reject moves it to rejected.
//
// current MUST be the store's authoritative thread state, never the state
// asserted on the received envelope (F3, WP-01 security review): a sender can
// sign an envelope claiming any state, so trusting it would let a forged
// "working" or "completed" bypass the human tap. A sender-chosen value is
// refused here — as ErrUnknownState if it is not a real state, or as
// ErrIllegalTransition if it is a real state other than input-required.
func ApplyInbound(current a2a.ThreadState, v Verdict) (a2a.ThreadState, error) {
	if !validState(current) {
		return "", ErrUnknownState
	}
	if current != a2a.StateInputRequired {
		return "", ErrIllegalTransition
	}
	switch v {
	case Approve:
		return a2a.StateWorking, nil
	case Reject:
		return a2a.StateRejected, nil
	default:
		return "", ErrIllegalTransition
	}
}

// validState reports whether s is one of the seven A2A thread states. The gate
// validates independently — it never accepts a state just because it arrived
// signed. Kept here rather than in a2a because validity is the gate's
// enforcement concern and a2a stays a pure data package; promote it to a2a if
// another package ever needs it.
func validState(s a2a.ThreadState) bool {
	switch s {
	case a2a.StateSubmitted, a2a.StateWorking, a2a.StateInputRequired,
		a2a.StateCompleted, a2a.StateFailed, a2a.StateCanceled, a2a.StateRejected:
		return true
	default:
		return false
	}
}

// Release is proof that an outbound reply cleared the approval gate. Only
// ApplyOutbound(..., Approve) mints one; the delivery layer (WP-08) requires a
// non-nil *Release to transmit. Its fields are unexported and it has no other
// constructor, so a reply cannot be delivered without having passed the gate —
// the guarantee is on the act of sending, not the "sent" label.
type Release struct {
	thread   string
	id       string
	payload  any
	viaGrant bool
}

// Thread reports the thread this release belongs to.
func (r *Release) Thread() string { return r.thread }

// ID reports the approved message's id. Delivery (WP-08) must match this AND
// Thread() against the message it transmits — thread alone would let an
// approved Release for one message send a different one in the same thread.
func (r *Release) ID() string { return r.id }

// Payload reports the approved payload (the envelope in v1).
func (r *Release) Payload() any { return r.payload }

// ViaGrant reports whether the release came from a standing grant rather than a
// fresh human tap. It is set by the grant path (subtask 4); a manual approval
// leaves it false.
func (r *Release) ViaGrant() bool { return r.viaGrant }

// ApplyOutbound applies a person's verdict to a drafted reply and returns the
// draft state it moves to. It is legal only from PendingReview: Approve mints a
// Release (the capability the delivery layer requires to transmit) and moves the
// draft to Sent; Reject moves it to Discarded and mints nothing.
//
// cur MUST be the store's authoritative draft state, never a value derived from
// untrusted input — the same discipline ApplyInbound applies to thread state.
//
// a.Direction must be Outbound: the gate refuses a mistagged approvable with
// ErrWrongDirection rather than trusting caller-supplied fields to agree — an
// inbound message must never mint a Release.
func ApplyOutbound(a Approvable, cur DraftState, v Verdict) (DraftState, *Release, error) {
	if a.Direction != Outbound {
		return "", nil, ErrWrongDirection
	}
	if !validDraftState(cur) {
		return "", nil, ErrUnknownState
	}
	if cur != PendingReview {
		return "", nil, ErrIllegalTransition
	}
	switch v {
	case Approve:
		return Sent, &Release{thread: a.Thread, id: a.ID, payload: a.Payload}, nil
	case Reject:
		return Discarded, nil, nil
	default:
		return "", nil, ErrIllegalTransition
	}
}

// validDraftState reports whether s is one of the three draft states. Mirrors
// validState: unknown input is refused, never guessed.
func validDraftState(s DraftState) bool {
	switch s {
	case PendingReview, Sent, Discarded:
		return true
	default:
		return false
	}
}

// Grant is a standing, revocable authorization to auto-approve a thread in
// ONE direction (D-03 inbound, D-11 outbound). The store persists grants as
// immortal audit rows (WP-03) and owns revocation; the gate treats a Grant
// as data and never looks one up.
//
// Grant deliberately has no Kind field while KindMessage is the only kind —
// whichever WP introduces a second Kind must add Kind here and to Covers, or
// every thread grant silently widens to the new kind.
type Grant struct {
	Thread    string
	Direction Direction
	Revoked   bool
}

// Covers reports whether g currently auto-approves a: same thread, same
// direction, not revoked. A grant covers exactly one direction, so an inbound
// grant never short-circuits an outbound reply and vice versa. Only the two
// canonical directions can be granted; a zero-value or thread-less grant
// covers nothing.
func (g Grant) Covers(a Approvable) bool {
	return !g.Revoked && g.Thread != "" &&
		(g.Direction == Inbound || g.Direction == Outbound) &&
		g.Thread == a.Thread && g.Direction == a.Direction
}

// ApplyInboundGrant auto-approves an inbound message when g covers it — the
// grant-driven equivalent of a human ApplyInbound(cur, Approve). If g does
// not cover a, it returns ErrNoGrant and the caller must get a human verdict.
// The store records the resulting transition as via-grant.
//
// a.Direction must be Inbound — a mistagged approvable is refused with
// ErrWrongDirection even when a grant would cover it, so an "outbound" pair
// can never flip a thread state through this path.
func ApplyInboundGrant(g Grant, a Approvable, cur a2a.ThreadState) (a2a.ThreadState, error) {
	if a.Direction != Inbound {
		return "", ErrWrongDirection
	}
	if !g.Covers(a) {
		return "", ErrNoGrant
	}
	return ApplyInbound(cur, Approve)
}

// ApplyOutboundGrant auto-releases a drafted reply when g covers it, minting a
// Release marked ViaGrant. If g does not cover a, it returns ErrNoGrant. It
// needs no direction assert of its own: it composes ApplyOutbound, which
// refuses a mistagged approvable with ErrWrongDirection.
func ApplyOutboundGrant(g Grant, a Approvable, cur DraftState) (DraftState, *Release, error) {
	if !g.Covers(a) {
		return "", nil, ErrNoGrant
	}
	st, rel, err := ApplyOutbound(a, cur, Approve)
	if rel != nil {
		rel.viaGrant = true
	}
	return st, rel, err
}
