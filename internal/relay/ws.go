package relay

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// hub is the in-memory registry of live daemon WebSocket connections, keyed by
// person then device. It is the delivery/approval push fan-out target (WP-08);
// nothing authoritative lives here — the durable inbox (WP-03) is the source of
// truth, so a dropped connection loses only a nudge, re-sent on reconnect.
type hub struct {
	mu    sync.Mutex
	conns map[string]map[string]*websocket.Conn // personID -> deviceID -> conn
}

func newHub() *hub {
	return &hub{conns: make(map[string]map[string]*websocket.Conn)}
}

// add registers c under (person, device); a reconnect supersedes and closes any
// previous socket for the same device.
func (h *hub) add(person, device string, c *websocket.Conn) {
	h.mu.Lock()
	old := h.conns[person][device]
	byDevice := h.conns[person]
	if byDevice == nil {
		byDevice = make(map[string]*websocket.Conn)
		h.conns[person] = byDevice
	}
	byDevice[device] = c
	h.mu.Unlock()
	if old != nil {
		_ = old.CloseNow() // best-effort: superseded by a reconnect
	}
}

// remove drops (person, device) only if c is still the registered conn — a
// socket already superseded by a reconnect must not evict its replacement.
func (h *hub) remove(person, device string, c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if byDevice := h.conns[person]; byDevice != nil && byDevice[device] == c {
		delete(byDevice, device)
		if len(byDevice) == 0 {
			delete(h.conns, person)
		}
	}
}

// handleWS upgrades a daemon connection after authenticating its device
// credential (a distinct long-lived token, T-06) and re-checking the device is
// live (ActiveDeviceByID — the revocation kill switch, arch §4.3). It registers
// the socket in the hub and holds it open until the peer disconnects. Delivery
// push (subtask 3) and outbound submit (subtask 4) fill in the frame loop.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	tok, ok := bearerToken(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	person, device, err := s.issuer.VerifyDeviceCredential(tok, time.Now().UTC())
	if err != nil {
		s.log.Warn("ws: device credential rejected", "err", err) // reason logged, never returned (T-17/T-18)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if _, err := s.store.ActiveDeviceByID(device); err != nil {
		s.log.Warn("ws: device not active", "device_id", device) // revoked/unknown — no oracle to caller
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	c, err := websocket.Accept(w, r, nil)
	if err != nil {
		s.log.Warn("ws: accept failed", "err", err) // Accept already wrote the response
		return
	}
	s.hub.add(person, device, c)
	s.log.Info("ws: connected", "person_id", person, "device_id", device)
	defer func() {
		s.hub.remove(person, device, c)
		_ = c.CloseNow() // best-effort on teardown
		s.log.Info("ws: disconnected", "person_id", person, "device_id", device)
	}()

	// Hold open, detecting disconnect via read error. Subtasks 3/4 handle the
	// frames the daemon sends (acks, outbound envelopes).
	ctx := r.Context()
	for {
		if _, _, err := c.Read(ctx); err != nil {
			return
		}
	}
}

// bearerToken extracts a bearer token from the Authorization header.
func bearerToken(r *http.Request) (string, bool) {
	const p = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) <= len(p) || !strings.EqualFold(h[:len(p)], p) {
		return "", false
	}
	return h[len(p):], true
}
