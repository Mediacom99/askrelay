package gate

import (
	"errors"
	"testing"

	"github.com/Mediacom99/askrelay/internal/a2a"
)

func TestApplyInbound(t *testing.T) {
	cases := []struct {
		name    string
		current a2a.ThreadState
		verdict Verdict
		want    a2a.ThreadState
		wantErr error
	}{
		{"approve from input-required", a2a.StateInputRequired, Approve, a2a.StateWorking, nil},
		{"reject from input-required", a2a.StateInputRequired, Reject, a2a.StateRejected, nil},
		{"approve from working is illegal", a2a.StateWorking, Approve, "", ErrIllegalTransition},
		{"approve from submitted is illegal", a2a.StateSubmitted, Approve, "", ErrIllegalTransition},
		{"reject from completed is illegal", a2a.StateCompleted, Reject, "", ErrIllegalTransition},
		{"forged 'completed' cannot drive a transition", a2a.StateCompleted, Approve, "", ErrIllegalTransition},
		{"unknown state is refused", a2a.ThreadState("made-up"), Approve, "", ErrUnknownState},
		{"empty state is refused", a2a.ThreadState(""), Approve, "", ErrUnknownState},
		{"bogus verdict is refused", a2a.StateInputRequired, Verdict("maybe"), "", ErrIllegalTransition},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ApplyInbound(c.current, c.verdict)
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("err = %v, want %v", err, c.wantErr)
			}
			if got != c.want {
				t.Errorf("state = %q, want %q", got, c.want)
			}
		})
	}
}

func TestApplyOutbound(t *testing.T) {
	sample := Approvable{Kind: KindMessage, Direction: Outbound, Thread: "01ABC", ID: "01MSG", Payload: "reply-body"}
	cases := []struct {
		name    string
		current DraftState
		verdict Verdict
		want    DraftState
		wantRel bool
		wantErr error
	}{
		{"approve from pending mints a release", PendingReview, Approve, Sent, true, nil},
		{"reject from pending mints nothing", PendingReview, Reject, Discarded, false, nil},
		{"approve from sent is illegal", Sent, Approve, "", false, ErrIllegalTransition},
		{"reject from discarded is illegal", Discarded, Reject, "", false, ErrIllegalTransition},
		{"unknown state is refused", DraftState("made-up"), Approve, "", false, ErrUnknownState},
		{"empty state is refused", DraftState(""), Approve, "", false, ErrUnknownState},
		{"bogus verdict is refused", PendingReview, Verdict("maybe"), "", false, ErrIllegalTransition},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, rel, err := ApplyOutbound(sample, c.current, c.verdict)
			if !errors.Is(err, c.wantErr) {
				t.Fatalf("err = %v, want %v", err, c.wantErr)
			}
			if got != c.want {
				t.Errorf("state = %q, want %q", got, c.want)
			}
			if (rel != nil) != c.wantRel {
				t.Fatalf("release present = %v, want %v", rel != nil, c.wantRel)
			}
			if rel != nil {
				if rel.Thread() != sample.Thread || rel.ID() != sample.ID || rel.Payload() != sample.Payload {
					t.Errorf("release carries wrong data: thread=%q id=%q payload=%v", rel.Thread(), rel.ID(), rel.Payload())
				}
				if rel.ViaGrant() {
					t.Error("manual approval must not be marked via_grant")
				}
			}
		})
	}
}

// The three mistagged-direction attacks from the WP-02 quality pass: an
// approvable whose Direction field disagrees with the path it is routed to
// must be refused, even when a grant covers it or the pair is internally
// consistent.
func TestWrongDirectionRefused(t *testing.T) {
	if _, _, err := ApplyOutbound(Approvable{Kind: KindMessage, Direction: Inbound, Thread: "01ABC", ID: "01MSG", Payload: "x"}, PendingReview, Approve); !errors.Is(err, ErrWrongDirection) {
		t.Errorf("inbound-tagged approvable through ApplyOutbound: err = %v, want ErrWrongDirection", err)
	}
	// Security PoC: outbound-only grant + outbound-tagged approvable routed
	// through the INBOUND grant path must not flip the thread state.
	g := Grant{Thread: "01ABC", Direction: Outbound}
	a := Approvable{Kind: KindMessage, Direction: Outbound, Thread: "01ABC", ID: "01MSG", Payload: "x"}
	if _, err := ApplyInboundGrant(g, a, a2a.StateInputRequired); !errors.Is(err, ErrWrongDirection) {
		t.Errorf("outbound pair through ApplyInboundGrant: err = %v, want ErrWrongDirection", err)
	}
	// Test-pass PoC: self-consistent INBOUND pair routed through the outbound
	// grant path must not mint a Release.
	gi := Grant{Thread: "01ABC", Direction: Inbound}
	ai := Approvable{Kind: KindMessage, Direction: Inbound, Thread: "01ABC", ID: "01MSG", Payload: "x"}
	if _, rel, err := ApplyOutboundGrant(gi, ai, PendingReview); !errors.Is(err, ErrWrongDirection) || rel != nil {
		t.Errorf("inbound pair through ApplyOutboundGrant: err = %v, rel = %v; want ErrWrongDirection, nil", err, rel)
	}
}

func TestGrantCovers(t *testing.T) {
	in := Approvable{Kind: KindMessage, Direction: Inbound, Thread: "01ABC", Payload: "question"}
	cases := []struct {
		name  string
		grant Grant
		want  bool
	}{
		{"covers own thread and direction", Grant{Thread: "01ABC", Direction: Inbound}, true},
		{"other direction is not covered", Grant{Thread: "01ABC", Direction: Outbound}, false},
		{"other thread is not covered", Grant{Thread: "01XYZ", Direction: Inbound}, false},
		{"revoked grant covers nothing", Grant{Thread: "01ABC", Direction: Inbound, Revoked: true}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.grant.Covers(in); got != c.want {
				t.Errorf("Covers = %v, want %v", got, c.want)
			}
		})
	}
}

func TestApplyInboundGrant(t *testing.T) {
	in := Approvable{Kind: KindMessage, Direction: Inbound, Thread: "01ABC", Payload: "question"}
	g := Grant{Thread: "01ABC", Direction: Inbound}

	got, err := ApplyInboundGrant(g, in, a2a.StateInputRequired)
	if err != nil {
		t.Fatalf("covered grant: err = %v, want nil", err)
	}
	if got != a2a.StateWorking {
		t.Errorf("state = %q, want %q", got, a2a.StateWorking)
	}

	if _, err := ApplyInboundGrant(Grant{Thread: "01XYZ", Direction: Inbound}, in, a2a.StateInputRequired); !errors.Is(err, ErrNoGrant) {
		t.Errorf("uncovered grant: err = %v, want ErrNoGrant", err)
	}
	if _, err := ApplyInboundGrant(g, in, a2a.StateWorking); !errors.Is(err, ErrIllegalTransition) {
		t.Errorf("covered grant from working: err = %v, want ErrIllegalTransition", err)
	}
}

func TestApplyOutboundGrant(t *testing.T) {
	out := Approvable{Kind: KindMessage, Direction: Outbound, Thread: "01ABC", Payload: "reply-body"}
	g := Grant{Thread: "01ABC", Direction: Outbound}

	st, rel, err := ApplyOutboundGrant(g, out, PendingReview)
	if err != nil {
		t.Fatalf("covered grant: err = %v, want nil", err)
	}
	if st != Sent {
		t.Errorf("state = %q, want %q", st, Sent)
	}
	if rel == nil {
		t.Fatal("covered grant must mint a release")
	}
	if !rel.ViaGrant() {
		t.Error("grant-minted release must report ViaGrant() == true")
	}

	if _, rel, err := ApplyOutboundGrant(Grant{Thread: "01ABC", Direction: Inbound}, out, PendingReview); !errors.Is(err, ErrNoGrant) || rel != nil {
		t.Errorf("inbound grant on outbound draft: err = %v, rel = %v; want ErrNoGrant, nil", err, rel)
	}
	if _, rel, err := ApplyOutboundGrant(g, out, Sent); !errors.Is(err, ErrIllegalTransition) || rel != nil {
		t.Errorf("covered grant from sent: err = %v, rel = %v; want ErrIllegalTransition, nil", err, rel)
	}
}
