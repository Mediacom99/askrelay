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
	payload  any
	viaGrant bool
}

// Thread reports the thread this release belongs to.
func (r *Release) Thread() string { return r.thread }

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
func ApplyOutbound(a Approvable, cur DraftState, v Verdict) (DraftState, *Release, error) {
	if !validDraftState(cur) {
		return "", nil, ErrUnknownState
	}
	if cur != PendingReview {
		return "", nil, ErrIllegalTransition
	}
	switch v {
	case Approve:
		return Sent, &Release{thread: a.Thread, payload: a.Payload}, nil
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
