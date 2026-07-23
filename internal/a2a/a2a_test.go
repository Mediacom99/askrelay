package a2a

import "testing"

// TestStateNamesVerbatim guards the A2A-verbatim hyphenated wire spellings of
// the thread-state constants. These strings travel on the wire and appear in
// audit rows, so a typo here is a protocol break — pin them explicitly.
func TestStateNamesVerbatim(t *testing.T) {
	pairs := map[ThreadState]string{
		StateSubmitted:     "submitted",
		StateWorking:       "working",
		StateInputRequired: "input-required",
		StateCompleted:     "completed",
		StateFailed:        "failed",
		StateCanceled:      "canceled",
		StateRejected:      "rejected",
	}
	for got, want := range pairs {
		if string(got) != want {
			t.Errorf("state constant = %q, want %q", got, want)
		}
	}
}
