package gate_test

import (
	"testing"

	"github.com/Mediacom99/askrelay/internal/gate"
)

// Release's fields are unexported, so outside the gate package a composite
// literal with data (gate.Release{thread: …}) does not compile — the only
// forgery left to an outsider is new(gate.Release): non-nil but inert. This
// pins that residue. The delivery layer must therefore match Thread()
// against the message it is about to transmit, never just check non-nil
// (recorded as a WP-08 requirement in the plan).
func TestZeroReleaseIsInert(t *testing.T) {
	rel := new(gate.Release)
	if rel.Thread() != "" || rel.Payload() != nil || rel.ViaGrant() {
		t.Errorf("zero Release leaks data: thread=%q payload=%v viaGrant=%v",
			rel.Thread(), rel.Payload(), rel.ViaGrant())
	}
}
