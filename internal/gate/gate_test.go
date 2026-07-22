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
