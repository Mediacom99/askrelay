package relay

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/Mediacom99/askrelay/internal/envelope"
)

// signTimeout bounds how long a release waits for a device signature (T-21).
// The daemon is a local process on the author's own machine and approve_reply is
// already an interactive call; past this the release proceeds relay-attested.
const signTimeout = 3 * time.Second

// signWaiters correlates a sign_request with the reply that answers it, keyed by
// the envelope (draft) id — unique per release, so at most one waiter per
// in-flight release.
type signWaiters struct {
	mu sync.Mutex
	m  map[string]chan wsFrame
}

func newSignWaiters() *signWaiters { return &signWaiters{m: map[string]chan wsFrame{}} }

func (w *signWaiters) register(id string, buf int) chan wsFrame {
	ch := make(chan wsFrame, buf)
	w.mu.Lock()
	w.m[id] = ch
	w.mu.Unlock()
	return ch
}

func (w *signWaiters) done(id string) {
	w.mu.Lock()
	delete(w.m, id)
	w.mu.Unlock()
}

// deliver hands a reply to its waiter, dropping it if nobody is waiting or the
// buffer is full — a late or unsolicited reply must never block a read loop.
func (w *signWaiters) deliver(id string, f wsFrame) {
	w.mu.Lock()
	ch := w.m[id]
	w.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- f:
	default:
	}
}

// SignRelease asks the author's connected devices to sign e and returns the
// first signature that verifies against that device's stored public key.
//
// ok=false means nobody signed in time — no daemon connected, a daemon that
// refused because it never forwarded this draft, or a signature that failed
// verification — and the caller delivers relay-attested (T-21: a missing daemon
// degrades the attestation, it never blocks the mail).
//
// The envelope handed to each device names THAT device in From.Device (T-21), so
// a signature is bound to the key that made it, and the relay verifies every
// signature itself rather than trusting the daemon's word.
func (s *Server) SignRelease(ctx context.Context, person string, e envelope.Envelope) (device string, sig []byte, ok bool) {
	conns := s.hub.snapshot(person)
	if len(conns) == 0 {
		return "", nil, false
	}
	ch := s.signWait.register(e.ID, len(conns))
	defer s.signWait.done(e.ID)

	sent := 0
	for dev, c := range conns {
		candidate := e
		candidate.From.Device = dev
		blob, err := json.Marshal(candidate)
		if err != nil {
			s.log.Error("sign: marshal candidate", "err", err, "message_id", e.ID)
			continue
		}
		select {
		case c.out <- wsFrame{Type: "sign_request", ID: e.ID, Envelope: blob}:
			sent++
		default: // that device's queue is full: skip it rather than stall a release
			s.log.Warn("sign: request queue full", "device_id", dev, "message_id", e.ID)
		}
	}
	if sent == 0 {
		return "", nil, false
	}

	timeout := time.After(signTimeout)
	for range sent { // at most one reply per device we asked
		select {
		case f := <-ch:
			if f.Type != "signature" || len(f.Sig) == 0 {
				continue // an explicit refusal: this device did not write that body
			}
			dev, err := s.store.ActiveDeviceByID(f.Device)
			if err != nil || dev.PersonID != person {
				// A signature from a revoked device, or from someone else's
				// device answering another person's release.
				s.log.Warn("sign: reply from an ineligible device", "device_id", f.Device, "message_id", e.ID)
				continue
			}
			candidate := e
			candidate.From.Device = f.Device
			candidate.Sig = f.Sig
			if err := envelope.Verify(candidate, dev.PubKey); err != nil {
				s.log.Warn("sign: offered signature did not verify", "device_id", f.Device, "message_id", e.ID)
				continue
			}
			return f.Device, f.Sig, true
		case <-timeout:
			return "", nil, false
		case <-ctx.Done():
			return "", nil, false
		}
	}
	return "", nil, false
}
