package gate

import (
	"errors"
	"testing"

	"github.com/Mediacom99/askrelay/internal/a2a"
)

// The sweeps prove the gate by exhaustion instead of by example: every
// state × verdict (and grant × state) combination is driven through the
// transition functions, and an independently-stated oracle says which few
// may succeed. A transition the example tests missed has nowhere to hide.

var (
	sweepThreadStates = []a2a.ThreadState{
		a2a.StateSubmitted, a2a.StateWorking, a2a.StateInputRequired,
		a2a.StateCompleted, a2a.StateFailed, a2a.StateCanceled, a2a.StateRejected,
		"", "made-up", "Working", "input_required", " input-required",
	}
	sweepDraftStates = []DraftState{
		PendingReview, Sent, Discarded,
		"", "made-up", "Sent", "pending review",
	}
	sweepVerdicts = []Verdict{Approve, Reject, "", "maybe", "APPROVE"}
	// sweepDirections is the approvable-direction axis — the one the WP-02
	// quality pass caught the original sweeps holding fixed.
	sweepDirections = []Direction{Inbound, Outbound, ""}
)

func TestInboundSweep(t *testing.T) {
	for _, s := range sweepThreadStates {
		for _, v := range sweepVerdicts {
			got, err := ApplyInbound(s, v)
			if s == a2a.StateInputRequired && (v == Approve || v == Reject) {
				want := a2a.StateWorking
				if v == Reject {
					want = a2a.StateRejected
				}
				if err != nil || got != want {
					t.Errorf("(%q, %q) = (%q, %v), want (%q, nil)", s, v, got, err, want)
				}
				continue
			}
			if err == nil {
				t.Errorf("(%q, %q) transitioned to %q; must refuse", s, v, got)
				continue
			}
			if got != "" {
				t.Errorf("(%q, %q) returned state %q alongside an error", s, v, got)
			}
			if !errors.Is(err, ErrUnknownState) && !errors.Is(err, ErrIllegalTransition) {
				t.Errorf("(%q, %q) err = %v, not a gate sentinel", s, v, err)
			}
		}
	}
}

func TestOutboundSweep(t *testing.T) {
	for _, d := range sweepDirections {
		a := Approvable{Kind: KindMessage, Direction: d, Thread: "01ABC", ID: "01MSG", Payload: "reply"}
		for _, s := range sweepDraftStates {
			for _, v := range sweepVerdicts {
				got, rel, err := ApplyOutbound(a, s, v)
				legal := d == Outbound && s == PendingReview && (v == Approve || v == Reject)
				if wantRel := legal && v == Approve; (rel != nil) != wantRel {
					t.Errorf("(%q, %q, %q): release present = %v, want %v", d, s, v, rel != nil, wantRel)
				}
				if rel != nil && rel.ViaGrant() {
					t.Errorf("(%q, %q, %q): manual release marked via_grant", d, s, v)
				}
				if legal {
					want := Sent
					if v == Reject {
						want = Discarded
					}
					if err != nil || got != want {
						t.Errorf("(%q, %q, %q) = (%q, %v), want (%q, nil)", d, s, v, got, err, want)
					}
					continue
				}
				if err == nil {
					t.Errorf("(%q, %q, %q) transitioned to %q; must refuse", d, s, v, got)
					continue
				}
				if d != Outbound && !errors.Is(err, ErrWrongDirection) {
					t.Errorf("(%q, %q, %q) err = %v, want ErrWrongDirection", d, s, v, err)
				}
				if got != "" {
					t.Errorf("(%q, %q, %q) returned state %q alongside an error", d, s, v, got)
				}
				if !errors.Is(err, ErrUnknownState) && !errors.Is(err, ErrIllegalTransition) && !errors.Is(err, ErrWrongDirection) {
					t.Errorf("(%q, %q, %q) err = %v, not a gate sentinel", d, s, v, err)
				}
			}
		}
	}
}

// sweepGrants is every grant shape: each thread (matching, foreign, empty) ×
// each direction (both real ones, empty) × revoked or not.
func sweepGrants() []Grant {
	var gs []Grant
	for _, th := range []string{"01ABC", "01XYZ", ""} {
		for _, d := range []Direction{Inbound, Outbound, ""} {
			for _, r := range []bool{false, true} {
				gs = append(gs, Grant{Thread: th, Direction: d, Revoked: r})
			}
		}
	}
	return gs
}

// The grant sweeps pin that the grant paths are EXACTLY "Covers, then the
// human Approve path" — never Reject, never a transition Approve could not
// make. Covers itself is oracle here, so its own truth table (including the
// zero-value case) is pinned separately in TestGrantCovers /
// TestZeroGrantCoversNothing.

func TestGrantSweepInbound(t *testing.T) {
	for _, d := range sweepDirections {
		a := Approvable{Kind: KindMessage, Direction: d, Thread: "01ABC", ID: "01MSG", Payload: "question"}
		for _, g := range sweepGrants() {
			for _, s := range sweepThreadStates {
				got, err := ApplyInboundGrant(g, a, s)
				if d != Inbound {
					if !errors.Is(err, ErrWrongDirection) || got != "" {
						t.Errorf("dir %q grant %+v: got (%q, %v), want (\"\", ErrWrongDirection)", d, g, got, err)
					}
					continue
				}
				if !g.Covers(a) {
					if !errors.Is(err, ErrNoGrant) || got != "" {
						t.Errorf("grant %+v uncovered: got (%q, %v), want (\"\", ErrNoGrant)", g, got, err)
					}
					continue
				}
				wantSt, wantErr := ApplyInbound(s, Approve)
				if got != wantSt || !errors.Is(err, wantErr) {
					t.Errorf("grant %+v state %q: got (%q, %v), want (%q, %v)", g, s, got, err, wantSt, wantErr)
				}
			}
		}
	}
}

func TestGrantSweepOutbound(t *testing.T) {
	for _, d := range sweepDirections {
		a := Approvable{Kind: KindMessage, Direction: d, Thread: "01ABC", ID: "01MSG", Payload: "reply"}
		for _, g := range sweepGrants() {
			for _, s := range sweepDraftStates {
				got, rel, err := ApplyOutboundGrant(g, a, s)
				if !g.Covers(a) {
					if !errors.Is(err, ErrNoGrant) || got != "" || rel != nil {
						t.Errorf("dir %q grant %+v uncovered: got (%q, %v, %v), want (\"\", nil, ErrNoGrant)", d, g, got, rel, err)
					}
					continue
				}
				// The oracle is literally the human path — which now embeds
				// the direction assert, so a covered-but-mistagged pair must
				// refuse with ErrWrongDirection on both sides of the compare.
				wantSt, wantRel, wantErr := ApplyOutbound(a, s, Approve)
				if got != wantSt || !errors.Is(err, wantErr) || (rel != nil) != (wantRel != nil) {
					t.Errorf("dir %q grant %+v state %q: got (%q, rel=%v, %v), want (%q, rel=%v, %v)",
						d, g, s, got, rel != nil, err, wantSt, wantRel != nil, wantErr)
				}
				if rel != nil && !rel.ViaGrant() {
					t.Errorf("grant %+v state %q: minted release not marked via_grant", g, s)
				}
			}
		}
	}
}

func TestZeroGrantCoversNothing(t *testing.T) {
	if (Grant{}).Covers(Approvable{}) {
		t.Error("zero-value grant must not cover a zero-value approvable")
	}
}
