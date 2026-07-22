package gate

import (
	"errors"

	"github.com/Mediacom99/askrelay/internal/a2a"
)

// Sentinel errors from the gate's transition functions. Callers branch on these
// with errors.Is; they are the gate's whole vocabulary of refusal.
var (
	// ErrUnknownState means the supplied current state is not one of the seven
	// A2A states. The gate refuses rather than guesses — the F3 guard: a state
	// it does not recognize is never a basis for a transition.
	ErrUnknownState = errors.New("gate: unknown thread state")
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
